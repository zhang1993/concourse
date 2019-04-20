package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func WorkerArtifacts(artifacts []db.WorkerArtifact) []atc.WorkerArtifact {
	wa := []atc.WorkerArtifact{}
	for _, a := range artifacts {
		wa = append(wa, WorkerArtifact(a))
	}
	return wa
}

// TODO: reduce the number to artifact structs
func WorkerArtifact(dbArtifact db.WorkerArtifact) atc.WorkerArtifact {
	return atc.WorkerArtifact{
		ID:        dbArtifact.ID(),
		Name:      dbArtifact.Name(),
		BuildID:   dbArtifact.BuildID(),
		CreatedAt: dbArtifact.CreatedAt().Unix(),
	}
}
