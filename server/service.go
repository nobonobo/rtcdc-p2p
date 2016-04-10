package main

import "net/rpc"

type Service struct{}

func (s *Service) Echo(req string, rep *string) error {
	*rep = req
	return nil
}

func init() {
	rpc.Register(&Service{})
}
