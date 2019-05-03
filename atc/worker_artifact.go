package atc

import "github.com/concourse/concourse/atc/worker"

type WorkerArtifact struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	BuildID   int    `json:"build_id"`
	CreatedAt int64  `json:"created_at"`
}

func NewWorkerArtifact(artifact worker.Artifact) WorkerArtifact {
	return WorkerArtifact{
		ID:        artifact.ID(),
		Name:      artifact.Name(),
		BuildID:   artifact.BuildID(),
		CreatedAt: artifact.CreatedAt(),
	}

}
