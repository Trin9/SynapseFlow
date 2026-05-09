package episode

// ReviewActionInput is the domain-layer input for human review decisions.
type ReviewActionInput struct {
	EpisodeID string
	Status    string
	Actor     string
	Note      string
}
