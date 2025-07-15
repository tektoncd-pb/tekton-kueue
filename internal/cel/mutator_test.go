package cel

import (
	"maps"
	"testing"

	. "github.com/onsi/gomega"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Common test constants to reduce duplication
const (
	complexPriorityExpression = `pacEventType == 'push' ? priority('push') :
				pacEventType == 'pull_request' ? priority('pull-request') :
				pacTestEventType == 'push' ? priority('push') :
				pacTestEventType == 'pull_request' ? priority('pull-request') :
				plrNamespace == 'rhtap-releng-tenant' ? priority('release') :
				plrNamespace == 'mintmaker' ? priority('dependency-update') :
				priority('default')`

	buildPlatformsExpression = `has(pipelineRun.spec.params) && pipelineRun.spec.params.exists(p, p.name == 'build-platforms') ?
				pipelineRun.spec.params.filter(p, p.name == 'build-platforms')[0].value.map(
					p,
					annotation("kueue.konflux-ci.dev/requests-" + replace(p, "/", "-"), "1") 
				) : []`
)

// Common test data helpers
func getBuildPlatformsParams() []tekv1.Param {
	return []tekv1.Param{
		{
			Name: "build-platforms",
			Value: tekv1.ParamValue{
				Type: tekv1.ParamTypeArray,
				ArrayVal: []string{
					"linux/arm64",
					"linux/amd64",
					"linux/s390x",
					"linux/ppc64le",
				},
			},
		},
	}
}

func getBuildPlatformsParamsSmall() []tekv1.Param {
	return []tekv1.Param{
		{
			Name: "build-platforms",
			Value: tekv1.ParamValue{
				Type: tekv1.ParamTypeArray,
				ArrayVal: []string{
					"linux/arm64",
					"linux/amd64",
				},
			},
		},
	}
}

func TestNewCELMutator(t *testing.T) {
	g := NewWithT(t)

	programs, err := CompileCELPrograms([]string{
		`annotation("test-key", "test-value")`,
		`label("env", "production")`,
	})
	g.Expect(err).NotTo(HaveOccurred())

	mutator := NewCELMutator(programs)

	g.Expect(mutator).NotTo(BeNil())
	g.Expect(mutator.programs).To(HaveLen(2))
}

func TestCELMutator_Mutate(t *testing.T) {
	tests := []struct {
		name                string
		expressions         []string
		namespace           string // optional, defaults to "test-namespace"
		initialLabels       map[string]string
		initialAnnotations  map[string]string
		initialParams       []tekv1.Param // optional, for testing parameter access
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		expectErr           bool
		errMsg              string
	}{
		{
			name: "single annotation mutation",
			expressions: []string{
				`annotation("tekton.dev/pipeline", "my-pipeline")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels:     nil,
			expectedAnnotations: map[string]string{
				"tekton.dev/pipeline": "my-pipeline",
			},
			expectErr: false,
		},
		{
			name: "single label mutation",
			expressions: []string{
				`label("environment", "production")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"environment": "production",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "multiple mutations from single expression",
			expressions: []string{
				`[annotation("owner", "team-a"), label("env", "prod")]`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"env": "prod",
			},
			expectedAnnotations: map[string]string{
				"owner": "team-a",
			},
			expectErr: false,
		},
		{
			name: "multiple expressions",
			expressions: []string{
				`annotation("pipeline", "test-pipeline")`,
				`label("stage", "testing")`,
				`annotation("version", "1.0")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"stage": "testing",
			},
			expectedAnnotations: map[string]string{
				"pipeline": "test-pipeline",
				"version":  "1.0",
			},
			expectErr: false,
		},
		{
			name: "merge with existing labels and annotations",
			expressions: []string{
				`annotation("new-annotation", "new-value")`,
				`label("new-label", "new-value")`,
			},
			initialLabels: map[string]string{
				"existing-label": "existing-value",
			},
			initialAnnotations: map[string]string{
				"existing-annotation": "existing-value",
			},
			expectedLabels: map[string]string{
				"existing-label": "existing-value",
				"new-label":      "new-value",
			},
			expectedAnnotations: map[string]string{
				"existing-annotation": "existing-value",
				"new-annotation":      "new-value",
			},
			expectErr: false,
		},
		{
			name: "overwrite existing values",
			expressions: []string{
				`annotation("existing-annotation", "updated-value")`,
				`label("existing-label", "updated-value")`,
			},
			initialLabels: map[string]string{
				"existing-label": "old-value",
			},
			initialAnnotations: map[string]string{
				"existing-annotation": "old-value",
			},
			expectedLabels: map[string]string{
				"existing-label": "updated-value",
			},
			expectedAnnotations: map[string]string{
				"existing-annotation": "updated-value",
			},
			expectErr: false,
		},
		{
			name: "runtime error in expression",
			expressions: []string{
				`annotation("", "test-value")`, // empty key should cause error
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectErr:          true,
			errMsg:             "annotation key cannot be empty",
		},
		{
			name: "reference pipelineRun name",
			expressions: []string{
				`annotation("pipeline-name", pipelineRun.metadata.name)`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels:     nil,
			expectedAnnotations: map[string]string{
				"pipeline-name": "test-pipeline",
			},
			expectErr: false,
		},
		{
			name: "reference pipelineRun namespace",
			expressions: []string{
				`label("namespace", pipelineRun.metadata.namespace)`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"namespace": "test-namespace",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "reference pipelineRef name",
			expressions: []string{
				`annotation("pipeline-ref", pipelineRun.spec.pipelineRef.name)`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels:     nil,
			expectedAnnotations: map[string]string{
				"pipeline-ref": "test-pipeline",
			},
			expectErr: false,
		},
		{
			name: "combine pipelineRun fields",
			expressions: []string{
				`[
					annotation("full-name", pipelineRun.metadata.namespace + "/" + pipelineRun.metadata.name),
					label("pipeline-ref", pipelineRun.spec.pipelineRef.name)
				]`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"pipeline-ref": "test-pipeline",
			},
			expectedAnnotations: map[string]string{
				"full-name": "test-namespace/test-pipeline",
			},
			expectErr: false,
		},
		{
			name: "conditional expression based on pipelineRun fields",
			expressions: []string{
				`annotation("environment", pipelineRun.metadata.namespace == "test-namespace" ? "testing" : "production")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels:     nil,
			expectedAnnotations: map[string]string{
				"environment": "testing",
			},
			expectErr: false,
		},
		{
			name: "reference existing label from pipelineRun",
			expressions: []string{
				`annotation("copied-label", pipelineRun.metadata.labels["existing-label"])`,
			},
			initialLabels: map[string]string{
				"existing-label": "label-value",
			},
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"existing-label": "label-value",
			},
			expectedAnnotations: map[string]string{
				"copied-label": "label-value",
			},
			expectErr: false,
		},
		{
			name: "priority function with static value",
			expressions: []string{
				`priority("high")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"kueue.x-k8s.io/priority-class": "high",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "priority function with dynamic value from pipelineRun",
			expressions: []string{
				`priority(pipelineRun.metadata.namespace == "production" ? "high" : "low")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"kueue.x-k8s.io/priority-class": "low",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "priority function combined with other mutations",
			expressions: []string{
				`[
					priority("medium"),
					annotation("priority-set-by", "cel-mutator"),
					label("queue", "default")
				]`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"kueue.x-k8s.io/priority-class": "medium",
				"queue":                         "default",
			},
			expectedAnnotations: map[string]string{
				"priority-set-by": "cel-mutator",
			},
			expectErr: false,
		},
		{
			name: "complex priority expression - push event",
			expressions: []string{
				complexPriorityExpression,
			},
			initialLabels: map[string]string{
				"pipelinesascode.tekton.dev/event-type": "push",
			},
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"pipelinesascode.tekton.dev/event-type": "push",
				"kueue.x-k8s.io/priority-class":         "push",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "complex priority expression - pull request event",
			expressions: []string{
				complexPriorityExpression,
			},
			initialLabels: map[string]string{
				"pipelinesascode.tekton.dev/event-type": "pull_request",
			},
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"pipelinesascode.tekton.dev/event-type": "pull_request",
				"kueue.x-k8s.io/priority-class":         "pull-request",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "complex priority expression - test event",
			expressions: []string{
				complexPriorityExpression,
			},
			initialLabels: map[string]string{
				"pac.test.appstudio.openshift.io/event-type": "push",
			},
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"pac.test.appstudio.openshift.io/event-type": "push",
				"kueue.x-k8s.io/priority-class":              "push",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "complex priority expression - release namespace",
			expressions: []string{
				complexPriorityExpression,
			},
			namespace:          "rhtap-releng-tenant",
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"kueue.x-k8s.io/priority-class": "release",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "complex priority expression - dependency update namespace",
			expressions: []string{
				complexPriorityExpression,
			},
			namespace:          "mintmaker",
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"kueue.x-k8s.io/priority-class": "dependency-update",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "complex priority expression - default fallback",
			expressions: []string{
				complexPriorityExpression,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"kueue.x-k8s.io/priority-class": "default",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "build-platforms parameter mapping to resource requests",
			expressions: []string{
				buildPlatformsExpression,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			initialParams:      getBuildPlatformsParams(),
			expectedLabels:     nil,
			expectedAnnotations: map[string]string{
				"kueue.konflux-ci.dev/requests-linux-arm64":   "1",
				"kueue.konflux-ci.dev/requests-linux-amd64":   "1",
				"kueue.konflux-ci.dev/requests-linux-s390x":   "1",
				"kueue.konflux-ci.dev/requests-linux-ppc64le": "1",
			},
			expectErr: false,
		},
		{
			name: "build-platforms parameter missing - returns empty array",
			expressions: []string{
				buildPlatformsExpression,
			},
			initialLabels:       nil,
			initialAnnotations:  nil,
			initialParams:       nil, // No parameters - should return empty array
			expectedLabels:      nil,
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "multiple expressions with build-platforms and priority",
			expressions: []string{
				`priority("high")`,
				`annotation("build-tool", "tekton")`,
				`label("team", "platform")`,
				buildPlatformsExpression,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			initialParams:      getBuildPlatformsParamsSmall(),
			expectedLabels: map[string]string{
				"kueue.x-k8s.io/priority-class": "high",
				"team":                          "platform",
			},
			expectedAnnotations: map[string]string{
				"build-tool": "tekton",
				"kueue.konflux-ci.dev/requests-linux-arm64": "1",
				"kueue.konflux-ci.dev/requests-linux-amd64": "1",
			},
			expectErr: false,
		},
		{
			name: "accessing parameter with invalid name - should fail",
			expressions: []string{
				"annotation('test', pipelineRun.doesNotExist)",
			},
			initialLabels:       nil,
			initialAnnotations:  nil,
			initialParams:       nil, // No parameters - should return empty array
			expectedLabels:      nil,
			expectedAnnotations: nil,
			expectErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Determine namespace to use
			namespace := tt.namespace
			if namespace == "" {
				namespace = "test-namespace"
			}

			// Create PipelineRun with initial state
			pipelineRun := &tekv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pipeline",
					Namespace:   namespace,
					Labels:      maps.Clone(tt.initialLabels),
					Annotations: maps.Clone(tt.initialAnnotations),
				},
				Spec: tekv1.PipelineRunSpec{
					PipelineRef: &tekv1.PipelineRef{
						Name: "test-pipeline",
					},
					Params: tt.initialParams,
				},
			}

			// Compile programs and create mutator
			programs, err := CompileCELPrograms(tt.expressions)
			g.Expect(err).NotTo(HaveOccurred())

			mutator := NewCELMutator(programs)

			// Apply mutations
			err = mutator.Mutate(pipelineRun)

			// Check for expected errors
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				if tt.errMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			// Verify labels
			g.Expect(pipelineRun.Labels).To(Equal(tt.expectedLabels))

			// Verify annotations
			g.Expect(pipelineRun.Annotations).To(Equal(tt.expectedAnnotations))
		})
	}
}

func TestCELMutator_Mutate_NilPipelineRun(t *testing.T) {
	g := NewWithT(t)

	programs, err := CompileCELPrograms([]string{
		`annotation("test-key", "test-value")`,
	})
	g.Expect(err).NotTo(HaveOccurred())

	mutator := NewCELMutator(programs)
	err = mutator.Mutate(nil)

	g.Expect(err).To(HaveOccurred())
}

func TestCELMutator_EmptyPrograms(t *testing.T) {
	g := NewWithT(t)

	mutator := NewCELMutator([]*CompiledProgram{})

	pipelineRun := &tekv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "test-namespace",
		},
	}

	err := mutator.Mutate(pipelineRun)
	g.Expect(err).NotTo(HaveOccurred())

	// Should not crash or modify the PipelineRun
	g.Expect(pipelineRun.Labels).To(BeNil())
	g.Expect(pipelineRun.Annotations).To(BeNil())
}
