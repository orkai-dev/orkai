package k3s

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

func newFakeOrchestrator(objects ...runtime.Object) *Orchestrator {
	return &Orchestrator{
		client: fake.NewSimpleClientset(objects...),
		logger: fakeLogger(),
	}
}

func newOrchestratorWithJobWatch(ns, jobName string, conditions ...batchv1.JobCondition) *Orchestrator {
	client := fake.NewSimpleClientset()
	client.PrependWatchReactor("jobs", func(action ktesting.Action) (bool, watch.Interface, error) {
		if action.GetResource().Resource != "jobs" {
			return false, nil, nil
		}
		fw := watch.NewFakeWithChanSize(1, false)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: ns},
			Status:     batchv1.JobStatus{Conditions: conditions},
		}
		go fw.Add(job)
		return true, fw, nil
	})
	return &Orchestrator{client: client, logger: fakeLogger()}
}

func seedBuildJob(t *testing.T, o *Orchestrator, ns, jobName string, conditions ...batchv1.JobCondition) {
	t.Helper()
	ctx := context.Background()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ns,
		},
	}
	if len(conditions) > 0 {
		job.Status.Conditions = conditions
	}
	_, err := o.client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	require.NoError(t, err)
}

func completeJobCondition() batchv1.JobCondition {
	return batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}
}

func failedJobCondition(message string) batchv1.JobCondition {
	return batchv1.JobCondition{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: message}
}

func TestWaitForBuildJobComplete(t *testing.T) {
	const ns = "default"
	const jobName = "build-test-complete"
	o := newOrchestratorWithJobWatch(ns, jobName, completeJobCondition())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logs, err := o.waitForBuildJob(ctx, ns, jobName, nil)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestWaitForBuildJobFailed(t *testing.T) {
	const ns = "default"
	const jobName = "build-test-failed"
	o := newOrchestratorWithJobWatch(ns, jobName, failedJobCondition("kaniko exploded"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := o.waitForBuildJob(ctx, ns, jobName, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kaniko exploded")
}

func TestWaitForBuildJobContextCanceled(t *testing.T) {
	const ns = "default"
	const jobName = "build-test-cancel"
	o := newFakeOrchestrator()
	seedBuildJob(t, o, ns, jobName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		_, err = o.waitForBuildJob(ctx, ns, jobName, nil)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	require.ErrorIs(t, err, context.Canceled)
}

func TestWaitForBuildJobWithOnLogNoRace(t *testing.T) {
	const ns = "default"
	const jobName = "build-test-onlog"
	o := newOrchestratorWithJobWatch(ns, jobName, completeJobCondition())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	onLog := func(_ string) {
		wg.Add(1)
		defer wg.Done()
	}

	_, err := o.waitForBuildJob(ctx, ns, jobName, onLog)
	require.NoError(t, err)
	wg.Wait()
}

func TestCollectBuildLogsEmptyWhenNoPod(t *testing.T) {
	o := newFakeOrchestrator()
	logs := o.collectBuildLogs(context.Background(), "default", "missing-job")
	assert.Empty(t, logs)
}

func TestCollectBuildLogsAggregatesContainers(t *testing.T) {
	const ns = "default"
	const jobName = "build-test-logs"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-pod",
			Namespace: ns,
			Labels:    map[string]string{"job-name": jobName},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "git-clone"}},
			Containers:     []corev1.Container{{Name: "kaniko"}},
		},
	}
	o := newFakeOrchestrator(pod)
	logs := o.collectBuildLogs(context.Background(), ns, jobName)
	assert.Contains(t, logs, "=== git-clone ===")
	assert.Contains(t, logs, "=== kaniko ===")
}

func TestCancelBuildDeletesLabeledJobs(t *testing.T) {
	appID := uuid.New()
	app := &model.Application{
		BaseModel: model.BaseModel{ID: appID},
		Namespace: "team-a",
		Name:      "web",
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-web-abc",
			Namespace: "team-a",
			Labels: map[string]string{
				"orkai/app-id": appID.String(),
				"orkai/build":  "true",
			},
		},
	}
	o := newFakeOrchestrator(job)
	ctx := context.Background()

	require.NoError(t, o.CancelBuild(ctx, app))

	jobs, err := o.client.BatchV1().Jobs("team-a").List(ctx, metav1.ListOptions{
		LabelSelector: "orkai/app-id=" + appID.String() + ",orkai/build=true",
	})
	require.NoError(t, err)
	assert.Empty(t, jobs.Items)
}
