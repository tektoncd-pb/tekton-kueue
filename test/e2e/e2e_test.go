/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konflux-ci/tekton-queue/test/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	kapi "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	"sigs.k8s.io/kueue/pkg/controller/jobframework"
)

// namespace where the project is deployed in
const namespace = "tekton-kueue-system"

// serviceAccountName created for the project
const serviceAccountName = "tekton-kueue-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "tekton-kueue-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "tekton-kueue-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string
	var k8sClient client.Client
	nsName := "test-ns"

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func(ctx context.Context) {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		projectImage := os.Getenv("IMG")
		Expect(projectImage).ToNot(Equal(""), "IMG environment variable must be declared")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

		By("Creating a k8s client")
		k8sClient = getK8sClientOrDie(ctx)

		By(fmt.Sprintf("Creating a namespace: %s", nsName), func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsName,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Satisfy(func(err error) bool {
				return err == nil || kerrors.IsAlreadyExists(err)
			}
		})

		By("Deploying ResourceFlavoer, ClusterQueue and Local Queue", func() {
			cmd := exec.Command(
				"kubectl",
				"apply",
				"--server-side",
				"-n",
				nsName,
				"-f",
				"config/samples/kueue/kueue-resources.yaml",
			)
			Expect(utils.Run(cmd)).To(Succeed(), "Failed to apply kueue resources")
		})
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	plrTemplate := &tekv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pipeline",
			Namespace:    "test-ns",
		},
		Spec: tekv1.PipelineRunSpec{
			PipelineSpec: &tekv1.PipelineSpec{
				Tasks: []tekv1.PipelineTask{
					{
						Name: "hello-world",
						TaskSpec: &tekv1.EmbeddedTask{
							TaskSpec: tekv1.TaskSpec{
								Steps: []tekv1.Step{
									{
										Name:    "hello-world",
										Image:   "registry.access.redhat.com/ubi9/ubi-micro:latest",
										Command: []string{"echo", "hello-world"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command(
				"kubectl",
				"create",
				"clusterrolebinding",
				"--dry-run=client",
				"-o",
				"yaml",
				metricsRoleBindingName,
				"--clusterrole=tekton-kueue-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			crb, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to generate ClusterRoleBinding")

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(crb)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("validating that the ServiceMonitor for Prometheus is applied in the namespace")
			cmd = exec.Command("kubectl", "get", "ServiceMonitor", "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "ServiceMonitor should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		It("should provisioned cert-manager", func() {
			By("validating that cert-manager has the certificate Secret")
			verifyCertManager := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "secrets", "webhook-server-cert", "-n", namespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyCertManager).Should(Succeed())
		})

		It("should have CA injection for mutating webhooks", func() {
			By("checking CA injection for mutating webhooks")
			verifyCAInjection := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"mutatingwebhookconfigurations.admissionregistration.k8s.io",
					"tekton-kueue-mutating-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				mwhOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(mwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		// TODO: Customize the e2e test suite with scenarios specific to your project.
		// Consider applying sample/CR(s) and check their status and/or verifying
		// the reconciliation by using the metrics, i.e.:
		// metricsOutput := getMetricsOutput()
		// Expect(metricsOutput).To(ContainSubstring(
		//    fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"} 1`,
		//    strings.ToLower(<Kind>),
		// ))
	})

	Context("N pipelines complete successfully", Ordered, func() {
		// todo generate a new one

		plrCount := 5
		plrs := make([]*tekv1.PipelineRun, plrCount)

		It("Starts PipelineRuns", func(ctx context.Context) {
			for i := range plrCount {
				plr := plrTemplate.DeepCopy()
				Eventually(
					func() error {
						return k8sClient.Create(ctx, plr)
					},
					90*time.Second,
					3*time.Second,
				).Should(Succeed())
				plrs[i] = plr
			}

		})

		It("A matching workload was created for each PipelineRun", func(ctx context.Context) {
			for i := range plrCount {
				plr := plrs[i]
				Eventually(func() error {
					_, err := GetOwnedWorkload(k8sClient, plr, ctx)
					return err
				},
					15*time.Second,
					3*time.Second,
				).Should(Succeed())
			}
		})

		It("PipelineRuns were completed Successfully", func(ctx context.Context) {
			for i := range plrCount {
				key := plrs[i].GetNamespacedName()
				plr := &tekv1.PipelineRun{}
				Eventually(func() error {
					err := k8sClient.Get(ctx, key, plr)
					if err != nil {
						return err
					}
					condition := plr.Status.GetCondition(kapi.ConditionSucceeded)
					if condition == nil {
						return fmt.Errorf("Success condition for PipelinerRun %s is nil", plr.Name)
					}
					success := (condition.Reason == tekv1.PipelineRunReasonSuccessful.String()) ||
						(condition.Reason == tekv1.PipelineRunReasonCompleted.String())
					if !success {
						return fmt.Errorf("PipelineRun %s didn't succeed", plr.Name)
					}
					return nil
				},
					(15*plrCount)*int(time.Second),
					3*time.Second,
				).Should(Succeed())
			}
		})
	})

	Context("Pipeline is queued when resources are missing", func() {
		var plr *tekv1.PipelineRun
		It("PipelineRun is queued because lack of resources", func(ctx context.Context) {
			plr = plrTemplate.DeepCopy()
			plr.Annotations = map[string]string{
				"kueue.konflux-ci.dev/requests-memory": "2Gi",
			}
			Eventually(
				func() error {
					return k8sClient.Create(ctx, plr)
				},
				90*time.Second,
				3*time.Second,
			).Should(Succeed())
		})

		It("Large Pipelinerun is Pending", func(ctx context.Context) {
			Eventually(
				func() error {
					key := plr.GetNamespacedName()
					err := k8sClient.Get(ctx, key, plr)
					if err != nil {
						return err
					}
					if plr.Spec.Status != tekv1.PipelineRunSpecStatusPending {
						return fmt.Errorf("PipelineRun status is %s and not Pending", plr.Spec.Status)
					}

					return nil
				},
				30*time.Second,
				3*time.Second,
			).Should(Succeed())
		})

		It("A matching workload was created for the PipelineRun", func(ctx context.Context) {
			Eventually(func() error {
				wl, err := GetOwnedWorkload(k8sClient, plr, ctx)
				if err != nil {
					return err
				}
				cond := apimeta.FindStatusCondition(wl.Status.Conditions, kueue.WorkloadQuotaReserved)
				if cond == nil {
					return fmt.Errorf("Didn't find QuotaReserved condition for workload %s", wl.Name)
				}

				if cond.Status != metav1.ConditionFalse {
					return fmt.Errorf("QuotaReserved Condition status isn't false (Quota Was reserved while it should have failed)")
				}

				if !strings.Contains(cond.Message, "insufficient quota for memory") {
					return fmt.Errorf("Didn't find expected condition message")
				}

				return nil
			},
				15*time.Second,
				3*time.Second,
			).Should(Succeed())
		})
	})
})

func GetOwnedWorkload(k8sClient client.Client, plr *tekv1.PipelineRun, ctx context.Context) (*kueue.Workload, error) {
	wlList := &kueue.WorkloadList{}
	ownerKey := jobframework.GetOwnerKey(tekv1.SchemeGroupVersion.WithKind("PipelineRun"))
	err := k8sClient.List(
		ctx,
		wlList,
		client.MatchingFields{ownerKey: plr.Name},
	)
	if err != nil {
		return nil, err
	}
	if len(wlList.Items) != 1 {
		return nil, fmt.Errorf("found %d owners for the workload", len(wlList.Items))
	}
	wl := wlList.Items[0]
	hasOwner, err := controllerutil.HasOwnerReference(wl.OwnerReferences, plr, k8sClient.Scheme())
	if err != nil {
		return nil, err
	}
	if !hasOwner {
		return nil, fmt.Errorf("The workload owner doesn't match the pipelinerun %s", plr.Name)
	}
	return &wl, nil
}

func getK8sClientOrDie(ctx context.Context) client.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(tekv1.AddToScheme(scheme))
	utilruntime.Must(kueue.AddToScheme(scheme))

	cfg := ctrl.GetConfigOrDie()
	k8sCache, err := cache.New(cfg, cache.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred(), "failed to create cache")

	err = jobframework.SetupWorkloadOwnerIndex(
		ctx,
		k8sCache,
		tekv1.SchemeGroupVersion.WithKind("PipelineRun"),
	)

	Expect(err).ToNot(HaveOccurred(), "failed to setup indexer")

	k8sClient, err := client.New(
		cfg,
		client.Options{
			Cache:  &client.CacheOptions{Reader: k8sCache},
			Scheme: scheme,
		},
	)
	Expect(err).ToNot(HaveOccurred(), "failed to create client")

	go func() {
		err := k8sCache.Start(ctx)
		if err != nil {
			panic(err)
		}
	}()

	return k8sClient
}

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
