package pages

import "testing"

func TestCredentialsValidate(t *testing.T) {
	cases := []struct {
		name    string
		creds   Credentials
		static  bool
		wantErr bool
	}{
		{
			name:    "access-key mode with keys",
			creds:   Credentials{AccessKeyID: "AKIA", SecretAccessKey: "secret"},
			static:  true,
			wantErr: false,
		},
		{
			name:    "access-key mode missing secret",
			creds:   Credentials{AccessKeyID: "AKIA"},
			static:  true,
			wantErr: true,
		},
		{
			name:    "access-key mode missing both",
			creds:   Credentials{AuthMode: "access_key"},
			static:  true,
			wantErr: true,
		},
		{
			name:    "instance-role mode with no keys",
			creds:   Credentials{AuthMode: "instance_role"},
			static:  false,
			wantErr: false,
		},
		{
			name:    "instance-role mode ignores blank keys",
			creds:   Credentials{AuthMode: "instance_role", DefaultRegion: "us-east-1"},
			static:  false,
			wantErr: false,
		},
		{
			name:    "assume-role mode with role ARN",
			creds:   Credentials{AuthMode: "assume_role", RoleARN: "arn:aws:iam::123456789012:role/orkai"},
			static:  false,
			wantErr: false,
		},
		{
			name:    "assume-role mode missing role ARN",
			creds:   Credentials{AuthMode: "assume_role"},
			static:  false,
			wantErr: true,
		},
		{
			name:    "assume-role mode with static base keys",
			creds:   Credentials{AuthMode: "assume_role", RoleARN: "arn:aws:iam::123456789012:role/orkai", AccessKeyID: "AKIA", SecretAccessKey: "secret"},
			static:  false,
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.creds.UseStaticKeys(); got != tc.static {
				t.Errorf("UseStaticKeys() = %v, want %v", got, tc.static)
			}
			err := tc.creds.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestCredentialsUseAssumeRole(t *testing.T) {
	cases := []struct {
		mode string
		want bool
	}{
		{"", false},
		{"access_key", false},
		{"instance_role", false},
		{"assume_role", true},
	}
	for _, tc := range cases {
		if got := (Credentials{AuthMode: tc.mode}).UseAssumeRole(); got != tc.want {
			t.Errorf("UseAssumeRole(%q) = %v, want %v", tc.mode, got, tc.want)
		}
	}
}
