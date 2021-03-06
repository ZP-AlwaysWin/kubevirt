/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2021 Red Hat, Inc.
 *
 */

package nodelabeller

import (
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8sv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	kubevirtv1 "kubevirt.io/client-go/api/v1"

	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/pkg/testutils"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
	device_manager "kubevirt.io/kubevirt/pkg/virt-handler/device-manager"
	util "kubevirt.io/kubevirt/pkg/virt-handler/node-labeller/util"
)

var _ = Describe("Node-labeller ", func() {
	var nlController *NodeLabeller
	var virtClient *kubecli.MockKubevirtClient
	var stop chan struct{}
	var ctrl *gomock.Controller
	var kubeClient *fake.Clientset
	var mockQueue *testutils.MockWorkQueue
	var config *virtconfig.ClusterConfig

	addNode := func(node *v1.Node) {
		mockQueue.ExpectAdds(1)
		nlController.queue.Add(node)
		mockQueue.Wait()
	}

	BeforeEach(func() {
		stop = make(chan struct{})
		ctrl = gomock.NewController(GinkgoT())

		kubeClient = fake.NewSimpleClientset()

		virtClient = kubecli.NewMockKubevirtClient(ctrl)
		virtClient.EXPECT().CoreV1().Return(kubeClient.CoreV1()).AnyTimes()

		kv := &kubevirtv1.KubeVirt{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubevirt",
				Namespace: "kubevirt",
			},
			Spec: kubevirtv1.KubeVirtSpec{
				Configuration: kubevirtv1.KubeVirtConfiguration{
					ObsoleteCPUModels: util.DefaultObsoleteCPUModels,
					MinCPUModel:       "Penryn",
				},
			},
		}

		config, _, _, _ = testutils.NewFakeClusterConfigUsingKV(kv)

		prepareFileDomCapabilities()

		nlController, _ = NewNodeLabeller(&device_manager.DeviceController{}, config, virtClient, "testNode", k8sv1.NamespaceDefault)

		mockQueue = testutils.NewMockWorkQueue(nlController.queue)

		nlController.queue = mockQueue
	})

	It("should run node-labelling", func() {
		addNode(newNode("testNode"))
		kubeClient.Fake.PrependReactor("*", "nodes", func(action testing.Action) (handled bool, obj runtime.Object, err error) {
			update, _ := action.(testing.PatchAction)
			Expect(update.GetName()).To(Equal("testNode"), "names should equal")
			containCorrectLabel := strings.Contains(string(update.GetPatch()), "Penryn")
			Expect(containCorrectLabel).To(Equal(true), "labels should contain cpu model")
			return true, nil, nil
		})

		nlController.execute()
	})

	AfterEach(func() {
		close(stop)
	})
})

func newNode(name string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Labels:      make(map[string]string),
			Name:        name,
		},
		Spec: v1.NodeSpec{},
	}
}
