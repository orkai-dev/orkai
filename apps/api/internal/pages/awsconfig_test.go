package pages

import (
	"context"
	"testing"
)

func TestAWSConfigAccessKeyModeRequiresKeys(t *testing.T) {
	_, err := (Credentials{AuthMode: AuthAccessKey}).AWSConfig(context.Background(), "us-east-1")
	if err == nil {
		t.Fatal("expected error when access-key mode has no keys")
	}
}

func TestAWSConfigAssumeRoleWithoutBaseKeysUsesDefaultChain(t *testing.T) {
	// Cannot assert which credentials the default chain resolves to without AWS
	// env/IAM, but loading config must not fail when keys are omitted.
	_, err := (Credentials{
		AuthMode: AuthAssumeRole,
		RoleARN:  "arn:aws:iam::123456789012:role/orkai",
	}).AWSConfig(context.Background(), "us-east-1")
	if err != nil {
		t.Fatalf("AWSConfig() error = %v", err)
	}
}
