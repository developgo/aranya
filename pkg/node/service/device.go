package service

type DeviceService struct {
}

func NewDeviceService() *DeviceService {
	return &DeviceService{}
}

func (n *DeviceService) SyncDeviceInfo(server Device_SyncDeviceInfoServer) error {
	return nil
}
