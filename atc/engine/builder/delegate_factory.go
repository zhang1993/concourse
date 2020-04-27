package builder

import (
	"io"
	"strings"
	"unicode/utf8"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/vars"
)

func NewDelegateFactory(eventProcessor db.EventProcessor) *delegateFactory {
	return &delegateFactory{
		eventProcessor: eventProcessor,
	}
}

type delegateFactory struct {
	eventProcessor db.EventProcessor
}

func (delegate *delegateFactory) GetDelegate(build db.Build, planID atc.PlanID, credVarsTracker vars.CredVarsTracker) exec.GetDelegate {
	return NewGetDelegate(build, planID, credVarsTracker, clock.NewClock(), delegate.eventProcessor)
}

func (delegate *delegateFactory) PutDelegate(build db.Build, planID atc.PlanID, credVarsTracker vars.CredVarsTracker) exec.PutDelegate {
	return NewPutDelegate(build, planID, credVarsTracker, clock.NewClock(), delegate.eventProcessor)
}

func (delegate *delegateFactory) TaskDelegate(build db.Build, planID atc.PlanID, credVarsTracker vars.CredVarsTracker) exec.TaskDelegate {
	return NewTaskDelegate(build, planID, credVarsTracker, clock.NewClock(), delegate.eventProcessor)
}

func (delegate *delegateFactory) CheckDelegate(check db.Check, planID atc.PlanID, credVarsTracker vars.CredVarsTracker) exec.CheckDelegate {
	return NewCheckDelegate(check, planID, credVarsTracker, clock.NewClock())
}

func (delegate *delegateFactory) BuildStepDelegate(build db.Build, planID atc.PlanID, credVarsTracker vars.CredVarsTracker) exec.BuildStepDelegate {
	return NewBuildStepDelegate(build, planID, credVarsTracker, clock.NewClock(), delegate.eventProcessor)
}

func NewGetDelegate(build db.Build, planID atc.PlanID, credVarsTracker vars.CredVarsTracker, clock clock.Clock, eventProcessor db.EventProcessor) exec.GetDelegate {
	return &getDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, credVarsTracker, clock, eventProcessor),

		eventProcessor: eventProcessor,
		eventOrigin:    event.Origin{ID: event.OriginID(planID)},
		build:          build,
		clock:          clock,
	}
}

type getDelegate struct {
	exec.BuildStepDelegate

	eventProcessor db.EventProcessor
	build          db.Build
	eventOrigin    event.Origin
	clock          clock.Clock
}

func (d *getDelegate) Initializing(logger lager.Logger) {
	err := d.eventProcessor.Process(d.build, event.InitializeGet{
		Origin: d.eventOrigin,
		Time:   d.clock.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-get-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *getDelegate) Starting(logger lager.Logger) {
	err := d.eventProcessor.Process(d.build, event.StartGet{
		Time:   d.clock.Now().Unix(),
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-get-event", err)
		return
	}

	logger.Info("starting")
}

func (d *getDelegate) Finished(logger lager.Logger, exitStatus exec.ExitStatus, info runtime.VersionResult) {
	// PR#4398: close to flush stdout and stderr
	d.Stdout().(io.Closer).Close()
	d.Stderr().(io.Closer).Close()

	err := d.eventProcessor.Process(d.build, event.FinishGet{
		Origin:          d.eventOrigin,
		Time:            d.clock.Now().Unix(),
		ExitStatus:      int(exitStatus),
		FetchedVersion:  info.Version,
		FetchedMetadata: info.Metadata,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-get-event", err)
		return
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}

func (d *getDelegate) UpdateVersion(log lager.Logger, plan atc.GetPlan, info runtime.VersionResult) {
	logger := log.WithData(lager.Data{
		"pipeline-name": d.build.PipelineName(),
		"pipeline-id":   d.build.PipelineID()},
	)

	pipeline, found, err := d.build.Pipeline()
	if err != nil {
		logger.Error("failed-to-find-pipeline", err)
		return
	}

	if !found {
		logger.Debug("pipeline-not-found")
		return
	}

	resource, found, err := pipeline.Resource(plan.Resource)
	if err != nil {
		logger.Error("failed-to-find-resource", err)
		return
	}

	if !found {
		logger.Debug("resource-not-found")
		return
	}

	_, err = resource.UpdateMetadata(
		info.Version,
		db.NewResourceConfigMetadataFields(info.Metadata),
	)
	if err != nil {
		logger.Error("failed-to-save-resource-config-version-metadata", err)
		return
	}
}

func NewPutDelegate(build db.Build, planID atc.PlanID, credVarsTracker vars.CredVarsTracker, clock clock.Clock, eventProcessor db.EventProcessor) exec.PutDelegate {
	return &putDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, credVarsTracker, clock, eventProcessor),

		eventProcessor: eventProcessor,
		eventOrigin:    event.Origin{ID: event.OriginID(planID)},
		build:          build,
		clock:          clock,
	}
}

type putDelegate struct {
	exec.BuildStepDelegate

	eventProcessor db.EventProcessor
	build          db.Build
	eventOrigin    event.Origin
	clock          clock.Clock
}

func (d *putDelegate) Initializing(logger lager.Logger) {
	err := d.eventProcessor.Process(d.build, event.InitializePut{
		Origin: d.eventOrigin,
		Time:   d.clock.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-put-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *putDelegate) Starting(logger lager.Logger) {
	err := d.eventProcessor.Process(d.build, event.StartPut{
		Time:   d.clock.Now().Unix(),
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-put-event", err)
		return
	}

	logger.Info("starting")
}

func (d *putDelegate) Finished(logger lager.Logger, exitStatus exec.ExitStatus, info runtime.VersionResult) {
	// PR#4398: close to flush stdout and stderr
	d.Stdout().(io.Closer).Close()
	d.Stderr().(io.Closer).Close()

	err := d.eventProcessor.Process(d.build, event.FinishPut{
		Origin:          d.eventOrigin,
		Time:            d.clock.Now().Unix(),
		ExitStatus:      int(exitStatus),
		CreatedVersion:  info.Version,
		CreatedMetadata: info.Metadata,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-put-event", err)
		return
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus, "version-info": info})
}

func (d *putDelegate) SaveOutput(log lager.Logger, plan atc.PutPlan, source atc.Source, resourceTypes atc.VersionedResourceTypes, info runtime.VersionResult) {
	logger := log.WithData(lager.Data{
		"step":          plan.Name,
		"resource":      plan.Resource,
		"resource-type": plan.Type,
		"version":       info.Version,
	})

	err := d.build.SaveOutput(
		plan.Type,
		source,
		resourceTypes,
		info.Version,
		db.NewResourceConfigMetadataFields(info.Metadata),
		plan.Name,
		plan.Resource,
	)
	if err != nil {
		logger.Error("failed-to-save-output", err)
		return
	}
}

func NewTaskDelegate(build db.Build, planID atc.PlanID, credVarsTracker vars.CredVarsTracker, clock clock.Clock, eventProcessor db.EventProcessor) exec.TaskDelegate {
	return &taskDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, credVarsTracker, clock, eventProcessor),

		eventProcessor: eventProcessor,
		eventOrigin:    event.Origin{ID: event.OriginID(planID)},
		build:          build,
		clock:          clock,
	}
}

type taskDelegate struct {
	exec.BuildStepDelegate
	eventProcessor db.EventProcessor
	config         atc.TaskConfig
	build          db.Build
	eventOrigin    event.Origin
	clock          clock.Clock
}

func (d *taskDelegate) SetTaskConfig(config atc.TaskConfig) {
	d.config = config
}

func (d *taskDelegate) Initializing(logger lager.Logger) {
	err := d.eventProcessor.Process(d.build, event.InitializeTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *taskDelegate) Starting(logger lager.Logger) {
	err := d.eventProcessor.Process(d.build, event.StartTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Debug("starting")
}

func (d *taskDelegate) Finished(logger lager.Logger, exitStatus exec.ExitStatus) {
	// PR#4398: close to flush stdout and stderr
	d.Stdout().(io.Closer).Close()
	d.Stderr().(io.Closer).Close()

	err := d.eventProcessor.Process(d.build, event.FinishTask{
		ExitStatus: int(exitStatus),
		Time:       d.clock.Now().Unix(),
		Origin:     d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
		return
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}

func NewCheckDelegate(check db.Check, planID atc.PlanID, credVarsTracker vars.CredVarsTracker, clock clock.Clock) exec.CheckDelegate {
	return &checkDelegate{
		BuildStepDelegate: NewBuildStepDelegate(nil, planID, credVarsTracker, clock, nil),

		eventOrigin: event.Origin{ID: event.OriginID(planID)},
		check:       check,
		clock:       clock,
	}
}

type checkDelegate struct {
	exec.BuildStepDelegate

	check       db.Check
	eventOrigin event.Origin
	clock       clock.Clock
}

func (d *checkDelegate) SaveVersions(versions []atc.Version) error {
	return d.check.SaveVersions(versions)
}

type discardCloser struct {
}

func (d discardCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (d discardCloser) Close() error {
	return nil
}

func (*checkDelegate) Stdout() io.Writer                                 { return discardCloser{} }
func (*checkDelegate) Stderr() io.Writer                                 { return discardCloser{} }
func (*checkDelegate) ImageVersionDetermined(db.UsedResourceCache) error { return nil }
func (*checkDelegate) Errored(lager.Logger, string)                      { return }

func NewBuildStepDelegate(
	build db.Build,
	planID atc.PlanID,
	credVarsTracker vars.CredVarsTracker,
	clock clock.Clock,
	eventProcessor db.EventProcessor,
) *buildStepDelegate {
	return &buildStepDelegate{
		eventProcessor:  eventProcessor,
		build:           build,
		planID:          planID,
		clock:           clock,
		credVarsTracker: credVarsTracker,
		stdout:          nil,
		stderr:          nil,
	}
}

type buildStepDelegate struct {
	eventProcessor  db.EventProcessor
	build           db.Build
	planID          atc.PlanID
	clock           clock.Clock
	credVarsTracker vars.CredVarsTracker
	stderr          io.Writer
	stdout          io.Writer
}

func (delegate *buildStepDelegate) Variables() vars.CredVarsTracker {
	return delegate.credVarsTracker
}

func (delegate *buildStepDelegate) ImageVersionDetermined(resourceCache db.UsedResourceCache) error {
	return delegate.build.SaveImageResourceVersion(resourceCache)
}

type credVarsIterator struct {
	line string
}

func (it *credVarsIterator) YieldCred(name, value string) {
	for _, lineValue := range strings.Split(value, "\n") {
		lineValue = strings.TrimSpace(lineValue)
		// Don't consider a single char as a secret.
		if len(lineValue) > 1 {
			it.line = strings.Replace(it.line, lineValue, "((redacted))", -1)
		}
	}
}

func (delegate *buildStepDelegate) buildOutputFilter(str string) string {
	it := &credVarsIterator{line: str}
	delegate.credVarsTracker.IterateInterpolatedCreds(it)
	return it.line
}

func (delegate *buildStepDelegate) Stdout() io.Writer {
	if delegate.stdout == nil {
		if delegate.credVarsTracker.Enabled() {
			delegate.stdout = newLogEventWriterWithSecretRedaction(
				delegate.eventProcessor,
				delegate.build,
				event.Origin{
					Source: event.OriginSourceStdout,
					ID:     event.OriginID(delegate.planID),
				},
				delegate.clock,
				delegate.buildOutputFilter,
			)
		} else {
			delegate.stdout = newLogEventWriter(
				delegate.eventProcessor,
				delegate.build,
				event.Origin{
					Source: event.OriginSourceStdout,
					ID:     event.OriginID(delegate.planID),
				},
				delegate.clock,
			)
		}
	}
	return delegate.stdout
}

func (delegate *buildStepDelegate) Stderr() io.Writer {
	if delegate.stderr == nil {
		if delegate.credVarsTracker.Enabled() {
			delegate.stderr = newLogEventWriterWithSecretRedaction(
				delegate.eventProcessor,
				delegate.build,
				event.Origin{
					Source: event.OriginSourceStderr,
					ID:     event.OriginID(delegate.planID),
				},
				delegate.clock,
				delegate.buildOutputFilter,
			)
		} else {
			delegate.stderr = newLogEventWriter(
				delegate.eventProcessor,
				delegate.build,
				event.Origin{
					Source: event.OriginSourceStderr,
					ID:     event.OriginID(delegate.planID),
				},
				delegate.clock,
			)
		}
	}
	return delegate.stderr
}

func (delegate *buildStepDelegate) Initializing(logger lager.Logger) {
	err := delegate.eventProcessor.Process(delegate.build, event.Initialize{
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time: delegate.clock.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
		return
	}

	logger.Info("initializing")
}

func (delegate *buildStepDelegate) Starting(logger lager.Logger) {
	err := delegate.eventProcessor.Process(delegate.build, event.Start{
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time: delegate.clock.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
		return
	}

	logger.Debug("starting")
}

func (delegate *buildStepDelegate) Finished(logger lager.Logger, succeeded bool) {
	// PR#4398: close to flush stdout and stderr
	delegate.Stdout().(io.Closer).Close()
	delegate.Stderr().(io.Closer).Close()

	err := delegate.eventProcessor.Process(delegate.build, event.Finish{
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time:      delegate.clock.Now().Unix(),
		Succeeded: succeeded,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
		return
	}

	logger.Info("finished")
}

func (delegate *buildStepDelegate) Errored(logger lager.Logger, message string) {
	err := delegate.eventProcessor.Process(delegate.build, event.Error{
		Message: message,
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time: delegate.clock.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

func newLogEventWriter(eventProcessor db.EventProcessor, build db.Build, origin event.Origin, clock clock.Clock) io.WriteCloser {
	return &logEventWriter{
		eventProcessor: eventProcessor,
		build:          build,
		origin:         origin,
		clock:          clock,
	}
}

type logEventWriter struct {
	eventProcessor db.EventProcessor
	build          db.Build
	origin         event.Origin
	clock          clock.Clock
	dangling       []byte
}

func (writer *logEventWriter) Write(data []byte) (int, error) {
	text := writer.writeDangling(data)
	if text == nil {
		return len(data), nil
	}

	err := writer.saveLog(string(text))
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (writer *logEventWriter) writeDangling(data []byte) []byte {
	text := append(writer.dangling, data...)

	checkEncoding, _ := utf8.DecodeLastRune(text)
	if checkEncoding == utf8.RuneError {
		writer.dangling = text
		return nil
	}

	writer.dangling = nil
	return text
}

func (writer *logEventWriter) saveLog(text string) error {
	return writer.eventProcessor.Process(writer.build, event.Log{
		Time:    writer.clock.Now().Unix(),
		Payload: text,
		Origin:  writer.origin,
	})
}

func (writer *logEventWriter) Close() error {
	return nil
}

// TODO: turn secret redaction into a decorator-type deal (if feasible)
func newLogEventWriterWithSecretRedaction(eventProcessor db.EventProcessor, build db.Build, origin event.Origin, clock clock.Clock, filter exec.BuildOutputFilter) io.Writer {
	return &logEventWriterWithSecretRedaction{
		logEventWriter: logEventWriter{
			eventProcessor: eventProcessor,
			build:          build,
			origin:         origin,
			clock:          clock,
		},
		filter: filter,
	}
}

type logEventWriterWithSecretRedaction struct {
	logEventWriter
	filter exec.BuildOutputFilter
}

func (writer *logEventWriterWithSecretRedaction) Write(data []byte) (int, error) {
	var text []byte

	if data != nil {
		text = writer.writeDangling(data)
		if text == nil {
			return len(data), nil
		}
	} else {
		if writer.dangling == nil || len(writer.dangling) == 0 {
			return 0, nil
		}
		text = writer.dangling
	}

	payload := string(text)
	if data != nil {
		idx := strings.LastIndex(payload, "\n")
		if idx >= 0 && idx < len(payload) {
			// Cache content after the last new-line, and proceed contents
			// before the last new-line.
			writer.dangling = ([]byte)(payload[idx+1:])
			payload = payload[:idx+1]
		} else {
			// No new-line found, then cache the log.
			writer.dangling = text
			return len(data), nil
		}
	}

	payload = writer.filter(payload)
	err := writer.saveLog(payload)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (writer *logEventWriterWithSecretRedaction) Close() error {
	writer.Write(nil)
	return nil
}
