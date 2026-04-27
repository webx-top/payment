package unionpay

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/admpub/log"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

// AmountToYi 金额转分(银联单位: 分，整数)
func AmountToYi(amount float64) string {
	return strconv.FormatInt(int64(amount*100), 10)
}

// YiToAmount 分转金额
func YiToAmount(yiStr string) float64 {
	if len(yiStr) == 0 {
		return 0
	}
	yi, err := strconv.ParseFloat(yiStr, 64)
	if err != nil {
		return 0
	}
	return yi / 100
}

// MoneyFeeToString 金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return fmt.Sprintf("%.2f", moneyFee)
}

func defaultString(val, def string) string {
	if len(val) == 0 {
		return def
	}
	return val
}

// MappingRespCode 映射银联响应码到交易状态
// 文档: https://open.unionpay.com/tjweb/doc/read?id=262
func MappingRespCode(respCode string) string {
	switch respCode {
	case "00": // 成功
		return config.TradeStatusSuccess
	case "01": // 失败(发卡行拒付)
		return config.TradeStatusException
	case "03": // 失败(商户未开通)
		return config.TradeStatusException
	case "04": // 撤单成功
		return config.TradeStatusClosed
	case "05": // 渠道不受理
		return config.TradeStatusException
	case "06": // 失败(金额超限)
		return config.TradeStatusException
	case "10": // 受理中/待确认(需后续查询)
		return config.TradeStatusProcessing
	case "11": // 重复订单号
		return config.TradeStatusWaitBuyerPay
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundRespCode 映射退款响应码到状态
func MapRefundRespCode(respCode string) string {
	switch respCode {
	case "00":
		return config.TradeStatusSuccess
	case "10":
		return config.TradeStatusProcessing
	default:
		return config.TradeStatusException
	}
}

// GenerateSign 生成银联签名
// 签名规则: 对所有非空字段按 key 字典序排列，拼接后用私钥 RSA-SHA256 签名
func (a *UnionPay) GenerateSign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "signature" && k != "signMethod" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := params[k]
		if len(v) > 0 {
			parts = append(parts, k+"="+v)
		}
	}
	signData := strings.Join(parts, "&")

	privateKey := a.account.PrivateKey
	if len(privateKey) == 0 {
		log.Warn("unionpay: privateKey not configured, skipping sign")
		return ""
	}

	sig, err := rsaSignPKCS1v15(privateKey, []byte(signData))
	if err != nil {
		log.Errorf("unionpay: sign failed %v", err)
		return ""
	}
	return sig
}

// VerifyPayloadSign 验证回调参数签名
// 银联回调使用公钥验证签名
func (a *UnionPay) VerifyPayloadSign(payload map[string]string) error {
	receivedSig := payload["signature"]
	if len(receivedSig) == 0 {
		return fmt.Errorf("missing signature")
	}

	// 提取签名字段并重新拼接验签串
	keys := make([]string, 0, len(payload))
	for k := range payload {
		if k != "signature" && k != "signMethod" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := payload[k]
		if len(v) > 0 {
			parts = append(parts, k+"="+v)
		}
	}
	verifyData := strings.Join(parts, "&")

	publicKey := a.account.AppSecret // 银联公钥存放在 AppSecret 中
	if len(publicKey) == 0 {
		log.Warn("unionpay: public key not configured, skipping verify")
		return nil
	}

	err := rsaVerifyPKCS1v15(publicKey, []byte(verifyData), receivedSig)
	if err != nil {
		return fmt.Errorf("signature verification failed: %v", err)
	}
	return nil
}

// parseFormData 解析请求中的表单数据(支持 form-data / JSON 两种格式)
func parseFormData(ctx echo.Context) map[string]string {
	result := make(map[string]string)

	var jsonPayload map[string]string
	if err := ctx.MustBind(&jsonPayload); err == nil && len(jsonPayload) > 0 {
		for k, v := range jsonPayload {
			result[k] = v
		}
	}

	return result
}

// VerifySign 实现 payment.Driver 接口的签名验证方法
func (a *UnionPay) VerifySign(ctx echo.Context) error {
	payload := parseFormData(ctx)
	if len(payload) == 0 {
		return fmt.Errorf("empty request body")
	}
	return a.VerifyPayloadSign(payload)
}

// --- 内部 RSA 签名/验签工具函数 ---

// rsaSignPKCS1v15 使用私钥进行 RSA PKCS#1 v1.5 SHA256 签名
// 支持两种格式: PEM 字符串 和 base64 编码的 DER 密钥
func rsaSignPKCS1v15(privateKeyPem string, data []byte) (string, error) {
	keyBytes := decodeKey(privateKeyPem)
	if keyBytes == nil {
		return "", fmt.Errorf("invalid private key")
	}

	privKey, err := parsePrivateKey(keyBytes)
	if err != nil {
		return "", err
	}

	hashed := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(nil, privKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// rsaVerifyPKCS1v15 使用公钥验证 RSA PKCS#1 v1.5 SHA256 签名
func rsaVerifyPKCS1v15(publicKeyPem string, data []byte, signatureB64 string) error {
	keyBytes := decodeKey(publicKeyPem)
	if keyBytes == nil {
		return fmt.Errorf("invalid public key")
	}

	pubKey, err := parsePublicKey(keyBytes)
	if err != nil {
		return err
	}

	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return err
	}

	hashed := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], signature)
}

// decodeKey 尝试解码密钥(base64 → 原始字节)
func decodeKey(key string) []byte {
	// 尝试 base64 解码
	if decoded, err := base64.StdEncoding.DecodeString(key); err == nil && len(decoded) > 20 {
		return decoded
	}
	trimmed := strings.TrimSpace(key)
	trimmed = strings.TrimPrefix(trimmed, "-----BEGIN PUBLIC KEY-----")
	trimmed = strings.TrimSuffix(trimmed, "-----END PUBLIC KEY-----")
	trimmed = strings.TrimPrefix(trimmed, "-----BEGIN PRIVATE KEY-----")
	trimmed = strings.TrimSuffix(trimmed, "-----END PRIVATE KEY-----")
	trimmed = strings.TrimSpace(trimmed)
	if len(trimmed) > 20 {
		if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
			return decoded
		}
	}
	return []byte(key)
}

// parsePrivateKey 从 DER/PEM 字节解析 RSA 私钥
func parsePrivateKey(derBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(derBytes)
	if block != nil {
		derBytes = block.Bytes
	}

	pkcs8Key, pkcs8Err := x509.ParsePKCS8PrivateKey(derBytes)
	if pkcs8Err == nil {
		rsaKey, ok := pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return rsaKey, nil
	}

	pkcs1Key, pkcs1Err := x509.ParsePKCS1PrivateKey(derBytes)
	if pkcs1Err == nil {
		return pkcs1Key, nil
	}
	return nil, fmt.Errorf("failed to parse private key: pkcs8=%v, pkcs1=%v", pkcs8Err, pkcs1Err)
}

// parsePublicKey 从 DER/PEM 字节解析 RSA 公钥
func parsePublicKey(derBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(derBytes)
	if block != nil {
		derBytes = block.Bytes
	}

	pkixPub, pkixErr := x509.ParsePKIXPublicKey(derBytes)
	if pkixErr == nil {
		rsaPub, ok := pkixPub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA public key")
		}
		return rsaPub, nil
	}

	pkcs1Pub, pkcs1Err := x509.ParsePKCS1PublicKey(derBytes)
	if pkcs1Err == nil {
		return pkcs1Pub, nil
	}
	return nil, fmt.Errorf("failed to parse public key: pkix=%v, pkcs1=%v", pkixErr, pkcs1Err)
}
