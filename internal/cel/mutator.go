package cel

import tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

// CELMutator applies mutations to PipelineRun objects based on compiled CEL programs.
// It evaluates CEL expressions and applies the resulting mutations to modify
// PipelineRun labels and annotations.
//
// Example usage:
//
//	programs, err := CompileCELPrograms([]string{
//		`annotation("tekton.dev/pipeline", "my-pipeline")`,
//		`[label("env", "production"), annotation("owner", "team-a")]`,
//	})
//	if err != nil {
//		return err
//	}
//
//	mutator := &CELMutator{programs: programs}
//	err = mutator.Mutate(pipelineRun)
type CELMutator struct {
	programs []*CompiledProgram
}

// NewCELMutator creates a new CELMutator with the provided compiled programs.
// The programs will be evaluated in order when Mutate is called.
func NewCELMutator(programs []*CompiledProgram) *CELMutator {
	return &CELMutator{programs: programs}
}

// Mutate applies all configured CEL mutations to the provided PipelineRun.
// It evaluates each compiled program and applies the resulting mutations
// to the PipelineRun's labels and annotations.
//
// The PipelineRun is modified in-place. If any evaluation fails, the method
// returns an error and the PipelineRun may be partially modified.
//
// Parameters:
//   - pipelineRun: The PipelineRun to mutate. Must not be nil.
//
// Returns:
//   - error: Any error that occurred during evaluation or mutation
func (m *CELMutator) Mutate(pipelineRun *tekv1.PipelineRun) error {
	mutations, err := m.evaluate(pipelineRun)
	if err != nil {
		return err
	}

	for _, mutation := range mutations {
		pipelineRun = mutate(pipelineRun, mutation)
	}
	return nil
}

// evaluate runs all compiled programs against the PipelineRun and collects
// all resulting mutations. Programs are evaluated in order, and all mutations
// are collected before any are applied.
//
// Parameters:
//   - pipelineRun: The PipelineRun to evaluate against
//
// Returns:
//   - []MutationRequest: All mutations from all programs
//   - error: Any error that occurred during evaluation
func (m *CELMutator) evaluate(pipelineRun *tekv1.PipelineRun) ([]*MutationRequest, error) {
	var allMutations []*MutationRequest
	for _, program := range m.programs {
		mutations, err := program.Evaluate(pipelineRun)
		if err != nil {
			RecordEvaluationFailure()
			return nil, err
		}
		allMutations = append(allMutations, mutations...)
	}
	RecordEvaluationSuccess()
	return allMutations, nil
}

// mutate applies a single mutation to the PipelineRun's metadata.
// It handles both label and annotation mutations, creating the respective
// maps if they don't exist.
//
// Parameters:
//   - pipelineRun: The PipelineRun to mutate
//   - mutation: The mutation to apply
//
// Returns:
//   - *tekv1.PipelineRun: The modified PipelineRun (same instance)
func mutate(pipelineRun *tekv1.PipelineRun, mutation *MutationRequest) *tekv1.PipelineRun {
	switch mutation.Type {
	case MutationTypeLabel:
		if pipelineRun.Labels == nil {
			pipelineRun.Labels = make(map[string]string)
		}
		pipelineRun.Labels[mutation.Key] = mutation.Value
	case MutationTypeAnnotation:
		if pipelineRun.Annotations == nil {
			pipelineRun.Annotations = make(map[string]string)
		}
		pipelineRun.Annotations[mutation.Key] = mutation.Value
	}
	return pipelineRun
}
