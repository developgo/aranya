package service

type PodService struct {
}

func NewPodService() *PodService {
	return &PodService{}
}

func (p *PodService) SyncDevicePods(server Pod_SyncDevicePodsServer) error {
	status, err := server.Recv()
	if err != nil {
		return err
	}

	switch status.GetStatus().(type) {
	case *PodStatus_Created:
	case *PodStatus_Deleted:
	case *PodStatus_Info:
	case *PodStatus_Updated:
	}

	return nil
}
