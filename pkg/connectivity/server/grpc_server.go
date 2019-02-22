package server

type grpcServer struct {
}

func NewGRPCServer() *grpcServer {
	return &grpcServer{}
}

func (s *grpcServer) ListenAndServe() error {
	return nil
}
