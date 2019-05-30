package image

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/worker"
	w "github.com/concourse/concourse/atc/worker"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")

type imageFactory struct {
	imageResourceFetcherFactory ImageResourceFetcherFactory
}

func NewImageFactory(
	imageResourceFetcherFactory ImageResourceFetcherFactory,
) worker.ImageFactory {
	return &imageFactory{
		imageResourceFetcherFactory: imageResourceFetcherFactory,
	}
}

func (f *imageFactory) GetImage(
	logger lager.Logger,
	worker worker.Worker,
	imageSpec worker.ImageSpec,
	teamID int,
	delegate worker.ImageFetchingDelegate,
	resourceTypes creds.VersionedResourceTypes,
) (worker.Image, error) {
	if imageSpec.ImageArtifactSource != nil {
		return &imageProvidedByWorker{
			imageSpec: imageSpec,
			teamID:    teamID,
			worker:    worker,
		}, nil
	}

	// check if custom resource
	resourceType, found := resourceTypes.Lookup(imageSpec.ResourceType)
	if found {
		imageResourceFetcher := f.imageResourceFetcherFactory.NewImageResourceFetcher(
			worker,
			w.ImageResource{
				Type:   resourceType.Type,
				Source: resourceType.Source,
				Params: &resourceType.Params,
			},
			resourceType.Version,
			teamID,
			resourceTypes.Without(imageSpec.ResourceType),
			delegate,
		)

		return &imageFromResource{
			imageResourceFetcher: imageResourceFetcher,

			privileged: resourceType.Privileged,
			teamID:     teamID,
			worker:     worker,
		}, nil
	}

	if imageSpec.ImageResource != nil {
		var version atc.Version
		if imageSpec.ImageResource.Version != nil {
			version = *imageSpec.ImageResource.Version
		}

		imageResourceFetcher := f.imageResourceFetcherFactory.NewImageResourceFetcher(
			worker,
			*imageSpec.ImageResource,
			version,
			teamID,
			resourceTypes,
			delegate,
		)

		return &imageFromResource{
			imageResourceFetcher: imageResourceFetcher,

			privileged: imageSpec.Privileged,
			teamID:     teamID,
			worker:     worker,
		}, nil
	}

	if imageSpec.ResourceType != "" {
		return &imageFromBaseResourceType{
			worker:           worker,
			resourceTypeName: imageSpec.ResourceType,
			teamID:           teamID,
		}, nil
	}

	return &imageFromRootfsURI{
		url: imageSpec.ImageURL,
	}, nil
}
