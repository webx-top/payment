package config

// Device 设备类型
type Device string

func (a Device) String() string {
	return string(a)
}

// IsSupported 是否是支持的设备
func (a Device) IsSupported() bool {
	for _, v := range devices {
		if a == v {
			return true
		}
	}
	return false
}

// IsMobile 是否是移动设备
func (a Device) IsMobile() bool {
	for _, v := range mobileDevices {
		if a == v {
			return true
		}
	}
	return false
}

const (
	// App 在App支付
	App Device = `app`
	// Web 在电脑端网页上支付
	Web Device = `web`
	// Wap 在手机端网页上支付
	Wap Device = `wap`
)

var (
	devices       = []Device{App, Web, Wap}
	mobileDevices = []Device{App, Wap}
)

func DeviceList() []Device {
	return devices
}
