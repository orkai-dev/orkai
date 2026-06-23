package k3s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

// resolveStorageClass returns the effective StorageClass for a PVC.
// override wins when set; otherwise the deploy-target default applies.
// nil means omit storageClassName and use the cluster default.
func (o *Orchestrator) resolveStorageClass(override string) *string {
	sc := override
	if sc == "" {
		sc = o.defaultStorageClass
	}
	if sc == "" {
		return nil
	}
	return &sc
}

func (o *Orchestrator) CreateVolume(ctx context.Context, opts orchestrator.VolumeOpts) (string, error) {
	qty, err := resource.ParseQuantity(opts.Size)
	if err != nil {
		return "", fmt.Errorf("invalid size %q: %w", opts.Size, err)
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    map[string]string{"managed-by": "orkai"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
		},
	}

	pvc.Spec.StorageClassName = o.resolveStorageClass(opts.StorageClass)

	_, err = o.client.CoreV1().PersistentVolumeClaims(opts.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", fmt.Errorf("create PVC: %w", err)
	}

	return opts.Name, nil
}

func (o *Orchestrator) DeleteVolume(ctx context.Context, name, namespace string) error {
	// Check if any pod is currently mounting this PVC
	pods, err := o.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, pod := range pods.Items {
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == name {
					return fmt.Errorf("PVC %s/%s is in use by pod %s — stop the workload first", namespace, name, pod.Name)
				}
			}
		}
	}

	if err := o.client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete PVC %s/%s: %w", namespace, name, err)
	}
	return nil
}

func (o *Orchestrator) ExpandVolume(ctx context.Context, name, namespace, newSize string) error {
	pvc, err := o.client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newQty, err := resource.ParseQuantity(newSize)
	if err != nil {
		return fmt.Errorf("invalid size %q: %w", newSize, err)
	}

	currentQty := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if newQty.Cmp(currentQty) <= 0 {
		return fmt.Errorf("new size %s must be larger than current size %s", newSize, currentQty.String())
	}

	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = newQty
	_, err = o.client.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("expand PVC %s/%s: %w", namespace, name, err)
	}
	return nil
}
