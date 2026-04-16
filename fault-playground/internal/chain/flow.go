package chain

import "context"

// FlowStep captures a single hop in the request chain.
type FlowStep struct {
	From ServiceName
	To   ServiceName
}

// FlowPlan enumerates the supported Phase 1 chains.
type FlowPlan struct {
	Name  string
	Steps []FlowStep
}

// BuildPhase1Plans returns the minimal plans used by the playground.
func BuildPhase1Plans() []FlowPlan {
	return []FlowPlan{
		{
			Name: "A->B->C",
			Steps: []FlowStep{
				{From: ServiceA, To: ServiceB},
				{From: ServiceB, To: ServiceC},
			},
		},
		{
			Name: "A->C->D->B->C",
			Steps: []FlowStep{
				{From: ServiceA, To: ServiceC},
				{From: ServiceC, To: ServiceD},
				{From: ServiceD, To: ServiceB},
				{From: ServiceB, To: ServiceC},
			},
		},
	}
}

// Runner orchestrates a flow plan using the Chain implementation.
type Runner struct {
	Chain Chain
}

// Run executes the named plan with the provided request.
func (r *Runner) Run(ctx context.Context, plan FlowPlan, req Request) (Response, error) {
	_ = ctx
	_ = plan
	return Response{}, nil
}
