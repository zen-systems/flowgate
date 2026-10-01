package plugin

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/go-plugin"
	pb "github.com/zen-systems/flowgate/pkg/plugin/proto"
	"google.golang.org/grpc"
)

// Handshake is a common handshake config for client and server.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "FLOWGATE_PLUGIN",
	MagicCookieValue: "flowgate_v1",
}

// FlowgatePluginImpl is the implementation of hashicorp/go-plugin mechanism.
type FlowgatePluginImpl struct {
	// Impl is the native Go interface implementation (for the server side).
	plugin.NetRPCUnsupportedPlugin
	Impl Plugin
}

// GRPCServer registers the gRPC server.
func (p *FlowgatePluginImpl) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterFlowgatePluginServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns the gRPC client.
func (p *FlowgatePluginImpl) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{Client: pb.NewFlowgatePluginClient(c)}, nil
}

// GRPCClient adapts the gRPC client to the Plugin interface.
type GRPCClient struct {
	Client pb.FlowgatePluginClient
}

func (c *GRPCClient) Manifest() Manifest {
	// In the simplified Protocol, manifest is loaded from file, not RPC.
	// But if we wanted dynamic capabilities, we'd add a GetManifest RPC.
	return Manifest{}
}

func (c *GRPCClient) Init(config map[string]interface{}) error {
	// Init handled via process env or constructor in gRPC usually.
	return nil
}

func (c *GRPCClient) Shutdown() error {
	return nil
}

func (c *GRPCClient) Analyze(ctx context.Context, req RequestContext) (*Result, error) {
	metadata := make(map[string]string)
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	protoReq := &pb.AnalyzeRequest{
		RunId:     req.RunID,
		Input:     req.Input,
		Artifacts: req.Artifacts,
		Metadata:  metadata,
	}

	resp, err := c.Client.Analyze(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return resultFromProto(resp), nil
}

// GRPCServer adapts the Plugin interface to the gRPC server.
type GRPCServer struct {
	pb.UnimplementedFlowgatePluginServer
	Impl Plugin
}

func (s *GRPCServer) Analyze(ctx context.Context, req *pb.AnalyzeRequest) (*pb.AnalyzeResult, error) {
	goReq := RequestContext{
		RunID:     req.RunId,
		Input:     req.Input,
		Artifacts: req.Artifacts,
		Metadata:  req.Metadata,
	}

	res, err := s.Impl.Analyze(ctx, goReq)
	if err != nil {
		return nil, err
	}

	return resultToProto(res), nil
}

// Helpers

func resultFromProto(res *pb.AnalyzeResult) *Result {
	r := &Result{
		Status:    Status(res.Status),
		Artifacts: res.Artifacts,
	}

	for _, att := range res.Attestations {
		r.Attestations = append(r.Attestations, Attestation{
			Subject: att.Subject,
			Hash:    att.Hash,
			Signer:  att.Signer,
			Claim:   unmarshalClaim(att.ClaimJson),
		})
	}
	// Diagnostics omitted for brevity
	return r
}

func resultToProto(res *Result) *pb.AnalyzeResult {
	p := &pb.AnalyzeResult{
		Status:    string(res.Status),
		Artifacts: res.Artifacts,
	}

	for _, att := range res.Attestations {
		p.Attestations = append(p.Attestations, &pb.Attestation{
			Subject:   att.Subject,
			Hash:      att.Hash,
			Signer:    att.Signer,
			ClaimJson: marshalClaim(att.Claim),
		})
	}
	return p
}

func marshalClaim(claim map[string]interface{}) string {
	b, _ := json.Marshal(claim)
	return string(b)
}

func unmarshalClaim(data string) map[string]interface{} {
	var m map[string]interface{}
	json.Unmarshal([]byte(data), &m)
	return m
}
