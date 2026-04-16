package configuration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// LoadSSMConfig loads parameters from SSM Parameter Store into environment
// variables. It is a no-op when not running in Lambda (AWS_LAMBDA_FUNCTION_NAME
// is unset).
func LoadSSMConfig(ctx context.Context) error {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		return nil
	}

	prefix := os.Getenv("SSM_PREFIX")
	if prefix == "" {
		return fmt.Errorf("SSM_PREFIX is required in Lambda")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	client := ssm.NewFromConfig(cfg)

	var (
		nextToken *string
		loaded    int
	)
	for {
		out, err := client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
			Path:           aws.String(prefix + "/"),
			WithDecryption: aws.Bool(true),
			NextToken:      nextToken,
		})
		if err != nil {
			return fmt.Errorf("fetching SSM parameters: %w", err)
		}

		for _, p := range out.Parameters {
			// /vibe-seeker/prod/DATABASE_URL → DATABASE_URL
			name := *p.Name
			name = name[strings.LastIndex(name, "/")+1:]
			if err := os.Setenv(name, *p.Value); err != nil {
				return fmt.Errorf("setting env var %s: %w", name, err)
			}
			slog.Debug("loaded SSM parameter", "name", name)
			loaded++
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	slog.Info("loaded SSM parameters", "count", loaded)
	return nil
}
