package controllers

import (
	"testing"

	"github.com/carina-io/carina/utils/iolimit"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Removing the annotations must still trigger a reconcile, which then resets io.max to max.
func TestRemovedAnnotationResetsLimit(t *testing.T) {
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1",
			Annotations:     map[string]string{"carina.storage.io/blkio.throttle.read_bps_device": "1048576"},
		},
		Spec:   corev1.PodSpec{NodeName: "node1"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	newPod := oldPod.DeepCopy()
	newPod.ResourceVersion = "2"
	newPod.Annotations = nil

	assert.True(t, podFilter{"node1"}.Update(event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}))

	r := &PodIOReconciler{}
	assert.Equal(t, &iolimit.IOLimit{Rbps: 1048576}, r.getPodIOLimit(oldPod))
	assert.Equal(t, &iolimit.IOLimit{}, r.getPodIOLimit(newPod))
}
