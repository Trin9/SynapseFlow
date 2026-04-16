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
	var resp Response
	var err error

	for _, step := range plan.Steps {
		switch step.To {
		case ServiceA:
			resp, err = r.Chain.CallA(ctx, req)
		case ServiceB:
			resp, err = r.Chain.CallB(ctx, req)
		case ServiceC:
			resp, err = r.Chain.CallC(ctx, req)
		case ServiceD:
			resp, err = r.Chain.CallD(ctx, req)
		default:
			// This case should ideally not be reached if FlowPlan is well-defined.
			return Response{}, nil
		}
		if err != nil {
			return Response{}, err
		}
		// Assuming the request payload might be updated by the response
		// or that intermediate responses are not needed until the end.
		// For now, we just pass the original request and return the last response.
	}
	return resp, nil
}
