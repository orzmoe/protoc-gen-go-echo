package runtime

import (
	grpcCodes "google.golang.org/grpc/codes"
)

// GRPCCodeToHTTPStatus 将 gRPC 状态码映射为 HTTP 状态码。
func GRPCCodeToHTTPStatus(c grpcCodes.Code) int {
	switch c {
	case grpcCodes.OK:
		return 200
	case grpcCodes.InvalidArgument, grpcCodes.FailedPrecondition, grpcCodes.OutOfRange:
		return 400
	case grpcCodes.Unauthenticated:
		return 401
	case grpcCodes.PermissionDenied:
		return 403
	case grpcCodes.NotFound:
		return 404
	case grpcCodes.AlreadyExists, grpcCodes.Aborted:
		return 409
	case grpcCodes.ResourceExhausted:
		return 429
	case grpcCodes.Canceled:
		return 499
	case grpcCodes.Unimplemented:
		return 501
	case grpcCodes.Unavailable:
		return 503
	case grpcCodes.DeadlineExceeded:
		return 504
	case grpcCodes.DataLoss, grpcCodes.Internal:
		return 500
	default:
		return 500
	}
}
