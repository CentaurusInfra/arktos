/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: builtins.proto

/*
Package mizar is a generated protocol buffer package.

It is generated from these files:
        builtins.proto

It has these top-level messages:
        BuiltinsNodeMessage
        BuiltinsPodMessage
        BuiltinsServiceMessage
        BuiltinsServiceEndpointMessage
        BuiltinsArktosMessage
        PortsMessage
        InterfacesMessage
        ReturnCode
*/
package mizar

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type CodeType int32

const (
	CodeType_OK         CodeType = 0
	CodeType_TEMP_ERROR CodeType = 1
	CodeType_PERM_ERROR CodeType = 2
)

var CodeType_name = map[int32]string{
	0: "OK",
	1: "TEMP_ERROR",
	2: "PERM_ERROR",
}
var CodeType_value = map[string]int32{
	"OK":         0,
	"TEMP_ERROR": 1,
	"PERM_ERROR": 2,
}

func (x CodeType) String() string {
	return proto.EnumName(CodeType_name, int32(x))
}
func (CodeType) EnumDescriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{0} }

type BuiltinsNodeMessage struct {
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Ip   string `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
}

func (m *BuiltinsNodeMessage) Reset()                    { *m = BuiltinsNodeMessage{} }
func (m *BuiltinsNodeMessage) String() string            { return proto.CompactTextString(m) }
func (*BuiltinsNodeMessage) ProtoMessage()               {}
func (*BuiltinsNodeMessage) Descriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{0} }

func (m *BuiltinsNodeMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *BuiltinsNodeMessage) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

type BuiltinsPodMessage struct {
	Name       string               `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	HostIp     string               `protobuf:"bytes,2,opt,name=host_ip,json=hostIp,proto3" json:"host_ip,omitempty"`
	Namespace  string               `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Tenant     string               `protobuf:"bytes,4,opt,name=tenant,proto3" json:"tenant,omitempty"`
	Vpc        string               `protobuf:"bytes,5,opt,name=vpc,proto3" json:"vpc,omitempty"`
	Phase      string               `protobuf:"bytes,6,opt,name=phase,proto3" json:"phase,omitempty"`
	Interfaces []*InterfacesMessage `protobuf:"bytes,7,rep,name=interfaces" json:"interfaces,omitempty"`
}

func (m *BuiltinsPodMessage) Reset()                    { *m = BuiltinsPodMessage{} }
func (m *BuiltinsPodMessage) String() string            { return proto.CompactTextString(m) }
func (*BuiltinsPodMessage) ProtoMessage()               {}
func (*BuiltinsPodMessage) Descriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{1} }

func (m *BuiltinsPodMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *BuiltinsPodMessage) GetHostIp() string {
	if m != nil {
		return m.HostIp
	}
	return ""
}

func (m *BuiltinsPodMessage) GetNamespace() string {
	if m != nil {
		return m.Namespace
	}
	return ""
}

func (m *BuiltinsPodMessage) GetTenant() string {
	if m != nil {
		return m.Tenant
	}
	return ""
}

func (m *BuiltinsPodMessage) GetVpc() string {
	if m != nil {
		return m.Vpc
	}
	return ""
}

func (m *BuiltinsPodMessage) GetPhase() string {
	if m != nil {
		return m.Phase
	}
	return ""
}

func (m *BuiltinsPodMessage) GetInterfaces() []*InterfacesMessage {
	if m != nil {
		return m.Interfaces
	}
	return nil
}

type BuiltinsServiceMessage struct {
	Name          string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	ArktosNetwork string `protobuf:"bytes,2,opt,name=arktos_network,json=arktosNetwork,proto3" json:"arktos_network,omitempty"`
	Namespace     string `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Tenant        string `protobuf:"bytes,4,opt,name=tenant,proto3" json:"tenant,omitempty"`
	Ip            string `protobuf:"bytes,5,opt,name=ip,proto3" json:"ip,omitempty"`
}

func (m *BuiltinsServiceMessage) Reset()                    { *m = BuiltinsServiceMessage{} }
func (m *BuiltinsServiceMessage) String() string            { return proto.CompactTextString(m) }
func (*BuiltinsServiceMessage) ProtoMessage()               {}
func (*BuiltinsServiceMessage) Descriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{2} }

func (m *BuiltinsServiceMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *BuiltinsServiceMessage) GetArktosNetwork() string {
	if m != nil {
		return m.ArktosNetwork
	}
	return ""
}

func (m *BuiltinsServiceMessage) GetNamespace() string {
	if m != nil {
		return m.Namespace
	}
	return ""
}

func (m *BuiltinsServiceMessage) GetTenant() string {
	if m != nil {
		return m.Tenant
	}
	return ""
}

func (m *BuiltinsServiceMessage) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

type BuiltinsServiceEndpointMessage struct {
	Name       string          `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	BackendIps []string        `protobuf:"bytes,2,rep,name=backend_ips,json=backendIps" json:"backend_ips,omitempty"`
	Ports      []*PortsMessage `protobuf:"bytes,3,rep,name=ports" json:"ports,omitempty"`
}

func (m *BuiltinsServiceEndpointMessage) Reset()         { *m = BuiltinsServiceEndpointMessage{} }
func (m *BuiltinsServiceEndpointMessage) String() string { return proto.CompactTextString(m) }
func (*BuiltinsServiceEndpointMessage) ProtoMessage()    {}
func (*BuiltinsServiceEndpointMessage) Descriptor() ([]byte, []int) {
	return fileDescriptorBuiltins, []int{3}
}

func (m *BuiltinsServiceEndpointMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *BuiltinsServiceEndpointMessage) GetBackendIps() []string {
	if m != nil {
		return m.BackendIps
	}
	return nil
}

func (m *BuiltinsServiceEndpointMessage) GetPorts() []*PortsMessage {
	if m != nil {
		return m.Ports
	}
	return nil
}

type BuiltinsArktosMessage struct {
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Vpc  string `protobuf:"bytes,2,opt,name=vpc,proto3" json:"vpc,omitempty"`
}

func (m *BuiltinsArktosMessage) Reset()                    { *m = BuiltinsArktosMessage{} }
func (m *BuiltinsArktosMessage) String() string            { return proto.CompactTextString(m) }
func (*BuiltinsArktosMessage) ProtoMessage()               {}
func (*BuiltinsArktosMessage) Descriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{4} }

func (m *BuiltinsArktosMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *BuiltinsArktosMessage) GetVpc() string {
	if m != nil {
		return m.Vpc
	}
	return ""
}

type PortsMessage struct {
	FrontendPort string `protobuf:"bytes,1,opt,name=frontend_port,json=frontendPort,proto3" json:"frontend_port,omitempty"`
	BackendPort  string `protobuf:"bytes,2,opt,name=backend_port,json=backendPort,proto3" json:"backend_port,omitempty"`
	Protocol     string `protobuf:"bytes,3,opt,name=protocol,proto3" json:"protocol,omitempty"`
}

func (m *PortsMessage) Reset()                    { *m = PortsMessage{} }
func (m *PortsMessage) String() string            { return proto.CompactTextString(m) }
func (*PortsMessage) ProtoMessage()               {}
func (*PortsMessage) Descriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{5} }

func (m *PortsMessage) GetFrontendPort() string {
	if m != nil {
		return m.FrontendPort
	}
	return ""
}

func (m *PortsMessage) GetBackendPort() string {
	if m != nil {
		return m.BackendPort
	}
	return ""
}

func (m *PortsMessage) GetProtocol() string {
	if m != nil {
		return m.Protocol
	}
	return ""
}

type InterfacesMessage struct {
	Name   string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Ip     string `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
	Subnet string `protobuf:"bytes,3,opt,name=subnet,proto3" json:"subnet,omitempty"`
}

func (m *InterfacesMessage) Reset()                    { *m = InterfacesMessage{} }
func (m *InterfacesMessage) String() string            { return proto.CompactTextString(m) }
func (*InterfacesMessage) ProtoMessage()               {}
func (*InterfacesMessage) Descriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{6} }

func (m *InterfacesMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *InterfacesMessage) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *InterfacesMessage) GetSubnet() string {
	if m != nil {
		return m.Subnet
	}
	return ""
}

type ReturnCode struct {
	Code    CodeType `protobuf:"varint,1,opt,name=code,proto3,enum=mizar.CodeType" json:"code,omitempty"`
	Message string   `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (m *ReturnCode) Reset()                    { *m = ReturnCode{} }
func (m *ReturnCode) String() string            { return proto.CompactTextString(m) }
func (*ReturnCode) ProtoMessage()               {}
func (*ReturnCode) Descriptor() ([]byte, []int) { return fileDescriptorBuiltins, []int{7} }

func (m *ReturnCode) GetCode() CodeType {
	if m != nil {
		return m.Code
	}
	return CodeType_OK
}

func (m *ReturnCode) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func init() {
	proto.RegisterType((*BuiltinsNodeMessage)(nil), "mizar.BuiltinsNodeMessage")
	proto.RegisterType((*BuiltinsPodMessage)(nil), "mizar.BuiltinsPodMessage")
	proto.RegisterType((*BuiltinsServiceMessage)(nil), "mizar.BuiltinsServiceMessage")
	proto.RegisterType((*BuiltinsServiceEndpointMessage)(nil), "mizar.BuiltinsServiceEndpointMessage")
	proto.RegisterType((*BuiltinsArktosMessage)(nil), "mizar.BuiltinsArktosMessage")
	proto.RegisterType((*PortsMessage)(nil), "mizar.PortsMessage")
	proto.RegisterType((*InterfacesMessage)(nil), "mizar.InterfacesMessage")
	proto.RegisterType((*ReturnCode)(nil), "mizar.ReturnCode")
	proto.RegisterEnum("mizar.CodeType", CodeType_name, CodeType_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for BuiltinsService service

type BuiltinsServiceClient interface {
	// For Services/Service Endpoints, network controller may want to annotate the Endpoints.
	// If Endpoints are not annotated, there will be many updates from unwanted endpoints.
	CreateService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	UpdateService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	ResumeService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	DeleteService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	UpdateServiceEndpoint(ctx context.Context, in *BuiltinsServiceEndpointMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	ResumeServiceEndpoint(ctx context.Context, in *BuiltinsServiceEndpointMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	CreateServiceEndpoint(ctx context.Context, in *BuiltinsServiceEndpointMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	ResumePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	UpdatePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	CreatePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	DeletePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	CreateNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	ResumeNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	UpdateNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	DeleteNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	CreateArktosNetwork(ctx context.Context, in *BuiltinsArktosMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	ResumeArktosNetwork(ctx context.Context, in *BuiltinsArktosMessage, opts ...grpc.CallOption) (*ReturnCode, error)
	UpdateArktosNetwork(ctx context.Context, in *BuiltinsArktosMessage, opts ...grpc.CallOption) (*ReturnCode, error)
}

type builtinsServiceClient struct {
	cc *grpc.ClientConn
}

func NewBuiltinsServiceClient(cc *grpc.ClientConn) BuiltinsServiceClient {
	return &builtinsServiceClient{cc}
}

func (c *builtinsServiceClient) CreateService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/CreateService", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) UpdateService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/UpdateService", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) ResumeService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/ResumeService", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) DeleteService(ctx context.Context, in *BuiltinsServiceMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/DeleteService", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) UpdateServiceEndpoint(ctx context.Context, in *BuiltinsServiceEndpointMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/UpdateServiceEndpoint", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) ResumeServiceEndpoint(ctx context.Context, in *BuiltinsServiceEndpointMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/ResumeServiceEndpoint", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) CreateServiceEndpoint(ctx context.Context, in *BuiltinsServiceEndpointMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/CreateServiceEndpoint", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) ResumePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/ResumePod", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) UpdatePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/UpdatePod", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) CreatePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/CreatePod", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) DeletePod(ctx context.Context, in *BuiltinsPodMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/DeletePod", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) CreateNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/CreateNode", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) ResumeNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/ResumeNode", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) UpdateNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/UpdateNode", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) DeleteNode(ctx context.Context, in *BuiltinsNodeMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/DeleteNode", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) CreateArktosNetwork(ctx context.Context, in *BuiltinsArktosMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/CreateArktosNetwork", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) ResumeArktosNetwork(ctx context.Context, in *BuiltinsArktosMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/ResumeArktosNetwork", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *builtinsServiceClient) UpdateArktosNetwork(ctx context.Context, in *BuiltinsArktosMessage, opts ...grpc.CallOption) (*ReturnCode, error) {
	out := new(ReturnCode)
	err := grpc.Invoke(ctx, "/mizar.BuiltinsService/UpdateArktosNetwork", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for BuiltinsService service

type BuiltinsServiceServer interface {
	// For Services/Service Endpoints, network controller may want to annotate the Endpoints.
	// If Endpoints are not annotated, there will be many updates from unwanted endpoints.
	CreateService(context.Context, *BuiltinsServiceMessage) (*ReturnCode, error)
	UpdateService(context.Context, *BuiltinsServiceMessage) (*ReturnCode, error)
	ResumeService(context.Context, *BuiltinsServiceMessage) (*ReturnCode, error)
	DeleteService(context.Context, *BuiltinsServiceMessage) (*ReturnCode, error)
	UpdateServiceEndpoint(context.Context, *BuiltinsServiceEndpointMessage) (*ReturnCode, error)
	ResumeServiceEndpoint(context.Context, *BuiltinsServiceEndpointMessage) (*ReturnCode, error)
	CreateServiceEndpoint(context.Context, *BuiltinsServiceEndpointMessage) (*ReturnCode, error)
	ResumePod(context.Context, *BuiltinsPodMessage) (*ReturnCode, error)
	UpdatePod(context.Context, *BuiltinsPodMessage) (*ReturnCode, error)
	CreatePod(context.Context, *BuiltinsPodMessage) (*ReturnCode, error)
	DeletePod(context.Context, *BuiltinsPodMessage) (*ReturnCode, error)
	CreateNode(context.Context, *BuiltinsNodeMessage) (*ReturnCode, error)
	ResumeNode(context.Context, *BuiltinsNodeMessage) (*ReturnCode, error)
	UpdateNode(context.Context, *BuiltinsNodeMessage) (*ReturnCode, error)
	DeleteNode(context.Context, *BuiltinsNodeMessage) (*ReturnCode, error)
	CreateArktosNetwork(context.Context, *BuiltinsArktosMessage) (*ReturnCode, error)
	ResumeArktosNetwork(context.Context, *BuiltinsArktosMessage) (*ReturnCode, error)
	UpdateArktosNetwork(context.Context, *BuiltinsArktosMessage) (*ReturnCode, error)
}

func RegisterBuiltinsServiceServer(s *grpc.Server, srv BuiltinsServiceServer) {
	s.RegisterService(&_BuiltinsService_serviceDesc, srv)
}

func _BuiltinsService_CreateService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsServiceMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).CreateService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/CreateService",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).CreateService(ctx, req.(*BuiltinsServiceMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_UpdateService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsServiceMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).UpdateService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/UpdateService",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).UpdateService(ctx, req.(*BuiltinsServiceMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_ResumeService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsServiceMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).ResumeService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/ResumeService",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).ResumeService(ctx, req.(*BuiltinsServiceMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_DeleteService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsServiceMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).DeleteService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/DeleteService",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).DeleteService(ctx, req.(*BuiltinsServiceMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_UpdateServiceEndpoint_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsServiceEndpointMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).UpdateServiceEndpoint(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/UpdateServiceEndpoint",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).UpdateServiceEndpoint(ctx, req.(*BuiltinsServiceEndpointMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_ResumeServiceEndpoint_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsServiceEndpointMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).ResumeServiceEndpoint(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/ResumeServiceEndpoint",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).ResumeServiceEndpoint(ctx, req.(*BuiltinsServiceEndpointMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_CreateServiceEndpoint_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsServiceEndpointMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).CreateServiceEndpoint(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/CreateServiceEndpoint",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).CreateServiceEndpoint(ctx, req.(*BuiltinsServiceEndpointMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_ResumePod_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsPodMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).ResumePod(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/ResumePod",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).ResumePod(ctx, req.(*BuiltinsPodMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_UpdatePod_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsPodMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).UpdatePod(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/UpdatePod",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).UpdatePod(ctx, req.(*BuiltinsPodMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_CreatePod_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsPodMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).CreatePod(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/CreatePod",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).CreatePod(ctx, req.(*BuiltinsPodMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_DeletePod_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsPodMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).DeletePod(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/DeletePod",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).DeletePod(ctx, req.(*BuiltinsPodMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_CreateNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsNodeMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).CreateNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/CreateNode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).CreateNode(ctx, req.(*BuiltinsNodeMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_ResumeNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsNodeMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).ResumeNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/ResumeNode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).ResumeNode(ctx, req.(*BuiltinsNodeMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_UpdateNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsNodeMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).UpdateNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/UpdateNode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).UpdateNode(ctx, req.(*BuiltinsNodeMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_DeleteNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsNodeMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).DeleteNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/DeleteNode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).DeleteNode(ctx, req.(*BuiltinsNodeMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_CreateArktosNetwork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsArktosMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).CreateArktosNetwork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/CreateArktosNetwork",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).CreateArktosNetwork(ctx, req.(*BuiltinsArktosMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_ResumeArktosNetwork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsArktosMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).ResumeArktosNetwork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/ResumeArktosNetwork",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).ResumeArktosNetwork(ctx, req.(*BuiltinsArktosMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuiltinsService_UpdateArktosNetwork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuiltinsArktosMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuiltinsServiceServer).UpdateArktosNetwork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mizar.BuiltinsService/UpdateArktosNetwork",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuiltinsServiceServer).UpdateArktosNetwork(ctx, req.(*BuiltinsArktosMessage))
	}
	return interceptor(ctx, in, info, handler)
}

var _BuiltinsService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "mizar.BuiltinsService",
	HandlerType: (*BuiltinsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateService",
			Handler:    _BuiltinsService_CreateService_Handler,
		},
		{
			MethodName: "UpdateService",
			Handler:    _BuiltinsService_UpdateService_Handler,
		},
		{
			MethodName: "ResumeService",
			Handler:    _BuiltinsService_ResumeService_Handler,
		},
		{
			MethodName: "DeleteService",
			Handler:    _BuiltinsService_DeleteService_Handler,
		},
		{
			MethodName: "UpdateServiceEndpoint",
			Handler:    _BuiltinsService_UpdateServiceEndpoint_Handler,
		},
		{
			MethodName: "ResumeServiceEndpoint",
			Handler:    _BuiltinsService_ResumeServiceEndpoint_Handler,
		},
		{
			MethodName: "CreateServiceEndpoint",
			Handler:    _BuiltinsService_CreateServiceEndpoint_Handler,
		},
		{
			MethodName: "ResumePod",
			Handler:    _BuiltinsService_ResumePod_Handler,
		},
		{
			MethodName: "UpdatePod",
			Handler:    _BuiltinsService_UpdatePod_Handler,
		},
		{
			MethodName: "CreatePod",
			Handler:    _BuiltinsService_CreatePod_Handler,
		},
		{
			MethodName: "DeletePod",
			Handler:    _BuiltinsService_DeletePod_Handler,
		},
		{
			MethodName: "CreateNode",
			Handler:    _BuiltinsService_CreateNode_Handler,
		},
		{
			MethodName: "ResumeNode",
			Handler:    _BuiltinsService_ResumeNode_Handler,
		},
		{
			MethodName: "UpdateNode",
			Handler:    _BuiltinsService_UpdateNode_Handler,
		},
		{
			MethodName: "DeleteNode",
			Handler:    _BuiltinsService_DeleteNode_Handler,
		},
		{
			MethodName: "CreateArktosNetwork",
			Handler:    _BuiltinsService_CreateArktosNetwork_Handler,
		},
		{
			MethodName: "ResumeArktosNetwork",
			Handler:    _BuiltinsService_ResumeArktosNetwork_Handler,
		},
		{
			MethodName: "UpdateArktosNetwork",
			Handler:    _BuiltinsService_UpdateArktosNetwork_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "builtins.proto",
}

func init() { proto.RegisterFile("builtins.proto", fileDescriptorBuiltins) }

var fileDescriptorBuiltins = []byte{
	// 654 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xb4, 0x56, 0xcd, 0x6e, 0xd3, 0x40,
	0x10, 0xae, 0x9d, 0x26, 0x6d, 0xa6, 0x69, 0x08, 0x5b, 0x5a, 0xac, 0xaa, 0x2a, 0xc5, 0xa5, 0x22,
	0xe2, 0x10, 0xa1, 0xc0, 0x85, 0x03, 0x87, 0x52, 0x72, 0x88, 0x50, 0x9a, 0xe0, 0x16, 0x24, 0x2e,
	0x44, 0x8e, 0xbd, 0xa5, 0x56, 0x92, 0xdd, 0xd5, 0xee, 0xa6, 0x08, 0x5e, 0x81, 0x27, 0xe0, 0xfd,
	0x78, 0x0e, 0x84, 0x76, 0xd7, 0xce, 0x2f, 0x09, 0x11, 0xb2, 0x6f, 0x9e, 0xf1, 0x37, 0xdf, 0xcc,
	0x7c, 0x33, 0xd6, 0x18, 0xca, 0xbd, 0x51, 0x34, 0x90, 0x11, 0x11, 0x35, 0xc6, 0xa9, 0xa4, 0xa8,
	0x32, 0x8c, 0xbe, 0xfb, 0x3c, 0xa0, 0x44, 0x72, 0x3a, 0x18, 0x60, 0x2e, 0xdc, 0x57, 0xb0, 0xf7,
	0x26, 0xc6, 0x5c, 0xd2, 0x10, 0xb7, 0xb0, 0x10, 0xfe, 0x17, 0x8c, 0x10, 0x6c, 0x12, 0x7f, 0x88,
	0x1d, 0xeb, 0xc4, 0xaa, 0x16, 0x3d, 0xfd, 0x8c, 0xca, 0x60, 0x47, 0xcc, 0xb1, 0xb5, 0xc7, 0x8e,
	0x98, 0xfb, 0xcb, 0x02, 0x94, 0xc4, 0x76, 0x68, 0xb8, 0x2a, 0xf4, 0x21, 0x6c, 0xdd, 0x52, 0x21,
	0xbb, 0xe3, 0xf8, 0x82, 0x32, 0x9b, 0x0c, 0x1d, 0x41, 0x51, 0x01, 0x04, 0xf3, 0x03, 0xec, 0xe4,
	0xf4, 0xab, 0x89, 0x03, 0x1d, 0x40, 0x41, 0x62, 0xe2, 0x13, 0xe9, 0x6c, 0x9a, 0x28, 0x63, 0xa1,
	0x0a, 0xe4, 0xee, 0x58, 0xe0, 0xe4, 0xb5, 0x53, 0x3d, 0xa2, 0x07, 0x90, 0x67, 0xb7, 0xbe, 0xc0,
	0x4e, 0x41, 0xfb, 0x8c, 0x81, 0x2e, 0x00, 0x22, 0x22, 0x31, 0xbf, 0xf1, 0x03, 0x2c, 0x9c, 0xad,
	0x93, 0x5c, 0x75, 0xa7, 0x7e, 0x5a, 0x9b, 0xd7, 0xa0, 0xd6, 0x1c, 0x63, 0xe2, 0x1e, 0xbc, 0xa9,
	0x30, 0xf7, 0xa7, 0x05, 0x07, 0x49, 0x9b, 0x57, 0x98, 0xdf, 0x45, 0xc1, 0x4a, 0x95, 0xce, 0xa0,
	0xec, 0xf3, 0xbe, 0xa4, 0xa2, 0x4b, 0xb0, 0xfc, 0x4a, 0x79, 0x3f, 0xee, 0x78, 0xd7, 0x78, 0x2f,
	0x8d, 0xf3, 0x3f, 0x1b, 0x37, 0x23, 0xc8, 0x8f, 0x47, 0xf0, 0xc3, 0x82, 0xe3, 0xb9, 0xda, 0x1a,
	0x24, 0x64, 0x34, 0x22, 0x72, 0x55, 0x8d, 0x8f, 0x60, 0xa7, 0xe7, 0x07, 0x7d, 0x4c, 0xc2, 0x6e,
	0xc4, 0x84, 0x63, 0x9f, 0xe4, 0xaa, 0x45, 0x0f, 0x62, 0x57, 0x93, 0x09, 0xf4, 0x12, 0xf2, 0x8c,
	0x72, 0x29, 0x9c, 0x9c, 0xd6, 0xec, 0x78, 0x51, 0xb3, 0x8e, 0x7a, 0x9d, 0xc8, 0x65, 0xc0, 0xee,
	0x6b, 0xd8, 0x4f, 0x8a, 0x39, 0xd7, 0xcd, 0xae, 0xaa, 0x21, 0x9e, 0xa1, 0x3d, 0x9e, 0xa1, 0xcb,
	0xa1, 0x34, 0xcd, 0x8a, 0x4e, 0x61, 0xf7, 0x86, 0x53, 0x22, 0x55, 0x99, 0x2a, 0x41, 0x1c, 0x5e,
	0x4a, 0x9c, 0x0a, 0x8c, 0x1e, 0x43, 0x29, 0x69, 0x45, 0x63, 0x0c, 0x5f, 0xd2, 0x9e, 0x86, 0x1c,
	0xc2, 0xb6, 0xde, 0xfe, 0x80, 0x0e, 0x62, 0xa5, 0xc7, 0xb6, 0xdb, 0x86, 0xfb, 0x0b, 0xd3, 0x5f,
	0x67, 0xf9, 0xd5, 0x84, 0xc4, 0xa8, 0x47, 0xb0, 0x8c, 0x29, 0x63, 0xcb, 0xfd, 0x08, 0xe0, 0x61,
	0x39, 0xe2, 0xe4, 0x82, 0x86, 0x18, 0xd5, 0x60, 0x33, 0xa0, 0xa1, 0x61, 0x2a, 0xd7, 0x0f, 0x17,
	0x65, 0x54, 0xa8, 0xeb, 0x6f, 0x0c, 0x7b, 0x1a, 0x87, 0x1c, 0xd8, 0x1a, 0x9a, 0x22, 0xe2, 0x54,
	0x89, 0xf9, 0xac, 0x0e, 0xdb, 0x09, 0x16, 0x15, 0xc0, 0x6e, 0xbf, 0xab, 0x6c, 0xa0, 0x32, 0xc0,
	0x75, 0xa3, 0xd5, 0xe9, 0x36, 0x3c, 0xaf, 0xed, 0x55, 0x2c, 0x65, 0x77, 0x1a, 0x5e, 0x2b, 0xb6,
	0xed, 0xfa, 0xef, 0x12, 0xdc, 0x9b, 0xdb, 0x0e, 0xf4, 0x09, 0x76, 0x2f, 0x38, 0xf6, 0x25, 0x4e,
	0x1c, 0xd5, 0xc5, 0xa2, 0xfe, 0xbe, 0xed, 0x87, 0x47, 0x8b, 0xc8, 0x49, 0xab, 0xee, 0x86, 0xa2,
	0xfe, 0xc0, 0xc2, 0xac, 0xa8, 0x3d, 0x2c, 0x46, 0xc3, 0x6c, 0xa8, 0xdf, 0xe2, 0x01, 0xce, 0xa2,
	0xea, 0x08, 0xf6, 0x67, 0x04, 0x49, 0x3e, 0x4d, 0xf4, 0xfc, 0x9f, 0x29, 0xe6, 0xbe, 0xe2, 0x75,
	0x52, 0xcd, 0x08, 0x94, 0x6d, 0xaa, 0x99, 0x0d, 0xca, 0x30, 0xd5, 0x7b, 0x28, 0x9a, 0xae, 0x3a,
	0x34, 0x44, 0x4f, 0x96, 0xd3, 0x4f, 0xae, 0xcf, 0x3a, 0x94, 0x66, 0x26, 0xa9, 0x52, 0x1a, 0x41,
	0x52, 0xa5, 0x34, 0x4b, 0x99, 0x1e, 0xe5, 0x15, 0x80, 0xa9, 0x52, 0x9d, 0x79, 0x74, 0xb6, 0x9c,
	0x73, 0xea, 0x37, 0x60, 0x1d, 0x52, 0x33, 0xa0, 0x94, 0x49, 0xcd, 0x88, 0x52, 0x26, 0x35, 0x8a,
	0xa6, 0x49, 0xfa, 0x19, 0xf6, 0x8c, 0xa6, 0xe7, 0x33, 0xb7, 0xfd, 0xe9, 0x72, 0xf6, 0x99, 0xbb,
	0xb8, 0x0e, 0xbf, 0x91, 0x37, 0x3b, 0x7e, 0xa3, 0x74, 0x36, 0xfc, 0xbd, 0x82, 0xbe, 0xb3, 0x2f,
	0xfe, 0x04, 0x00, 0x00, 0xff, 0xff, 0x68, 0x16, 0xfa, 0xd5, 0x87, 0x0a, 0x00, 0x00,
}
