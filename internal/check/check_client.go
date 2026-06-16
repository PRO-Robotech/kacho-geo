// Package check — per-RPC authz gate for kacho-geo. Wraps the corelib authz
// interceptor with the geo PermissionMap and an IAM-backed CheckClient
// (InternalIAMService.Check → OpenFGA/ReBAC). geo is a CONSUMER of iam authz
// (geo→iam Check edge — allowed; iam is the leaf-owner of authz).
package check

import (
	"context"

	"google.golang.org/grpc"

	"github.com/PRO-Robotech/kacho-corelib/auth"
	"github.com/PRO-Robotech/kacho-corelib/authz"
	iamv1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/iam/v1"
)

// IAMCheckClient adapts kacho-iam.InternalIAMService.Check to authz.CheckClient.
type IAMCheckClient struct {
	cli iamv1.InternalIAMServiceClient
}

// NewIAMCheckClient builds the adapter over a conn to kacho-iam internal (:9091).
func NewIAMCheckClient(conn grpc.ClientConnInterface) *IAMCheckClient {
	return &IAMCheckClient{cli: iamv1.NewInternalIAMServiceClient(conn)}
}

// Check calls InternalIAMService.Check. The outgoing ctx is wrapped with
// auth.PropagateOutgoing so the iam-side principal-extract sees the real caller.
func (c *IAMCheckClient) Check(ctx context.Context, subjectID, relation, object string) (bool, error) {
	resp, err := c.cli.Check(auth.PropagateOutgoing(ctx), &iamv1.CheckRequest{
		SubjectId: subjectID,
		Relation:  relation,
		Object:    object,
	})
	if err != nil {
		return false, err
	}
	return resp.GetAllowed(), nil
}

var _ authz.CheckClient = (*IAMCheckClient)(nil)
