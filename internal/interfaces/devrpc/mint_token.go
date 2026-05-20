package devrpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"connectrpc.com/connect"
	devv1 "github.com/cthierer/canterbury/gen/go/canterbury/dev/v1"
	"github.com/cthierer/canterbury/internal/domain/devauth"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MintToken handles development token minting requests.
func (service *DevAuthServiceHandler) MintToken(
	ctx context.Context,
	req *connect.Request[devv1.MintTokenRequest],
) (*connect.Response[devv1.MintTokenResponse], error) {
	claims, options, err := mintTokenRequestToDomain(req.Msg)
	if err != nil {
		connectErr := classifyMintTokenError(err)
		logConnectError(ctx, "encountered an error minting token", err, connectErr)
		return nil, connectErr
	}

	token, err := service.auth.MintToken(ctx, claims, options)
	if err != nil {
		connectErr := classifyMintTokenError(err)
		logConnectError(ctx, "encountered an error minting token", err, connectErr)
		return nil, connectErr
	}

	resp := connect.NewResponse(&devv1.MintTokenResponse{
		Token: mintedTokenToProto(token),
	})

	return resp, nil
}

// mintTokenRequestToDomain converts the wire request into application inputs.
// It only rejects values that cannot be safely represented before application
// validation, leaving claim normalization and token policy checks to devauth.
func mintTokenRequestToDomain(req *devv1.MintTokenRequest) (devauth.Claims, devauth.MintOptions, error) {
	options, err := mintOptionsFromProto(req.GetOptions())
	if err != nil {
		return devauth.Claims{}, devauth.MintOptions{}, err
	}

	claims := devauth.Claims{
		Subject:   req.GetClaims().GetSubject(),
		Audiences: req.GetClaims().GetAudiences(),
	}

	return claims, options, nil
}

// mintOptionsFromProto protects the int64 nanosecond time.Duration conversion.
// The application service still owns policy limits such as maximum token TTL.
func mintOptionsFromProto(options *devv1.MintTokenOptions) (devauth.MintOptions, error) {
	ttlSeconds := options.GetTtlSeconds()
	if ttlSeconds < 0 {
		return devauth.MintOptions{}, devauth.ErrNegativeTTL
	}

	maxDurationSeconds := math.MaxInt64 / int64(time.Second)
	if ttlSeconds > maxDurationSeconds {
		return devauth.MintOptions{}, devauth.ErrLargeTTL
	}

	return devauth.MintOptions{
		TTL: time.Duration(ttlSeconds) * time.Second,
	}, nil
}

// mintedTokenToProto converts a domain token into the public RPC response shape.
func mintedTokenToProto(token devauth.Token) *devv1.MintedToken {
	return &devv1.MintedToken{
		Jwt:       token.JWT,
		TokenType: token.Type.String(),
		ExpiresAt: timestamppb.New(token.ExpiresAt),
	}
}

// classifyMintTokenError translates application and system errors into Connect
// status errors while keeping implementation details out of client responses.
func classifyMintTokenError(err error) error {
	if errors.Is(err, devauth.ErrMissingSubject) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("subject is required"))
	}

	if errors.Is(err, devauth.ErrMissingAudience) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least 1 audience is required"))
	}

	if errors.Is(err, devauth.ErrNegativeTTL) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("ttl must not be negative"))
	}

	if errors.Is(err, devauth.ErrLargeTTL) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("ttl is out of range"))
	}

	if errors.Is(err, context.Canceled) {
		return connect.NewError(connect.CodeCanceled, fmt.Errorf("request canceled"))
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return connect.NewError(connect.CodeDeadlineExceeded, fmt.Errorf("deadline exceeded"))
	}

	return connect.NewError(connect.CodeUnknown, fmt.Errorf("an unknown error occurred"))
}
