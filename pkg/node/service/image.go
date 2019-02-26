package service

type ImageService struct {
}

func NewImageService() *ImageService {
	return &ImageService{}
}

func (m *ImageService) SyncDeviceImages(server Image_SyncDeviceImagesServer) error {
	return nil
}
