package k3s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

func TestResolveStorageClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		defaultClass string
		override     string
		want         *string
	}{
		{
			name:         "override wins",
			defaultClass: "gp3",
			override:     "io2",
			want:         strPtr("io2"),
		},
		{
			name:         "default when override empty",
			defaultClass: "gp3",
			override:     "",
			want:         strPtr("gp3"),
		},
		{
			name:         "nil when both empty",
			defaultClass: "",
			override:     "",
			want:         nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := &Orchestrator{defaultStorageClass: tc.defaultClass}
			got := o.resolveStorageClass(tc.override)
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, *tc.want, *got)
		})
	}
}

func TestEnsurePVCStorageClass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	labels := map[string]string{"managed-by": "orkai"}

	t.Run("uses default storage class", func(t *testing.T) {
		t.Parallel()
		o := &Orchestrator{
			client:              fake.NewSimpleClientset(),
			logger:              fakeLogger(),
			defaultStorageClass: "gp3",
		}
		require.NoError(t, o.ensurePVC(ctx, "default", "app-data", "5Gi", "", labels))

		pvc, err := o.client.CoreV1().PersistentVolumeClaims("default").Get(ctx, "app-data", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pvc.Spec.StorageClassName)
		assert.Equal(t, "gp3", *pvc.Spec.StorageClassName)
	})

	t.Run("omits storage class when default unset", func(t *testing.T) {
		t.Parallel()
		o := &Orchestrator{
			client: fake.NewSimpleClientset(),
			logger: fakeLogger(),
		}
		require.NoError(t, o.ensurePVC(ctx, "default", "app-data", "1Gi", "", labels))

		pvc, err := o.client.CoreV1().PersistentVolumeClaims("default").Get(ctx, "app-data", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, pvc.Spec.StorageClassName)
	})

	t.Run("override wins over default", func(t *testing.T) {
		t.Parallel()
		o := &Orchestrator{
			client:              fake.NewSimpleClientset(),
			logger:              fakeLogger(),
			defaultStorageClass: "gp3",
		}
		require.NoError(t, o.ensurePVC(ctx, "default", "app-io2", "5Gi", "io2", labels))

		pvc, err := o.client.CoreV1().PersistentVolumeClaims("default").Get(ctx, "app-io2", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pvc.Spec.StorageClassName)
		assert.Equal(t, "io2", *pvc.Spec.StorageClassName)
	})
}

func TestCreateVolumeStorageClass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	o := &Orchestrator{
		client:              fake.NewSimpleClientset(),
		logger:              fakeLogger(),
		defaultStorageClass: "gp3",
	}

	_, err := o.CreateVolume(ctx, orchestrator.VolumeOpts{
		Name:      "vol-1",
		Namespace: "default",
		Size:      "2Gi",
	})
	require.NoError(t, err)

	pvc, err := o.client.CoreV1().PersistentVolumeClaims("default").Get(ctx, "vol-1", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, pvc.Spec.StorageClassName)
	assert.Equal(t, "gp3", *pvc.Spec.StorageClassName)
}

func strPtr(s string) *string {
	return &s
}

func TestCreateVolumeOverrideStorageClass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	o := &Orchestrator{
		client:              fake.NewSimpleClientset(),
		logger:              fakeLogger(),
		defaultStorageClass: "gp3",
	}

	_, err := o.CreateVolume(ctx, orchestrator.VolumeOpts{
		Name:         "vol-2",
		Namespace:    "default",
		Size:         "2Gi",
		StorageClass: "io2",
	})
	require.NoError(t, err)

	pvc, err := o.client.CoreV1().PersistentVolumeClaims("default").Get(ctx, "vol-2", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, pvc.Spec.StorageClassName)
	assert.Equal(t, "io2", *pvc.Spec.StorageClassName)
}

func TestGetStorageClasses(t *testing.T) {
	t.Parallel()

	allowExpansion := true
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gp3",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner:          "ebs.csi.aws.com",
		AllowVolumeExpansion: &allowExpansion,
	}
	o := &Orchestrator{client: fake.NewSimpleClientset(sc), logger: fakeLogger()}

	classes, err := o.GetStorageClasses(context.Background())
	require.NoError(t, err)
	require.Len(t, classes, 1)
	assert.Equal(t, "gp3", classes[0].Name)
	assert.Equal(t, "ebs.csi.aws.com", classes[0].Provisioner)
	assert.True(t, classes[0].IsDefault)
	assert.True(t, classes[0].AllowExpansion)
}
