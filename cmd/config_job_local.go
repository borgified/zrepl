package cmd

import (
	"time"

	"context"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/zrepl/zrepl/rpc"
)

type LocalJob struct {
	Name              string
	Mapping           *DatasetMapFilter
	SnapshotFilter    *PrefixSnapshotFilter
	Interval          time.Duration
	InitialReplPolicy InitialReplPolicy
	PruneLHS          PrunePolicy
	PruneRHS          PrunePolicy
	Debug             JobDebugSettings
}

func parseLocalJob(name string, i map[string]interface{}) (j *LocalJob, err error) {

	var asMap struct {
		Mapping           map[string]string
		SnapshotPrefix    string `mapstructure:"snapshot_prefix"`
		Interval          string
		InitialReplPolicy string                 `mapstructure:"initial_repl_policy"`
		PruneLHS          map[string]interface{} `mapstructure:"prune_lhs"`
		PruneRHS          map[string]interface{} `mapstructure:"prune_rhs"`
		Debug             map[string]interface{}
	}

	if err = mapstructure.Decode(i, &asMap); err != nil {
		err = errors.Wrap(err, "mapstructure error")
		return nil, err
	}

	j = &LocalJob{Name: name}

	if j.Mapping, err = parseDatasetMapFilter(asMap.Mapping, false); err != nil {
		return
	}

	if j.SnapshotFilter, err = parsePrefixSnapshotFilter(asMap.SnapshotPrefix); err != nil {
		return
	}

	if j.Interval, err = time.ParseDuration(asMap.Interval); err != nil {
		err = errors.Wrap(err, "cannot parse interval")
		return
	}

	if j.InitialReplPolicy, err = parseInitialReplPolicy(asMap.InitialReplPolicy, DEFAULT_INITIAL_REPL_POLICY); err != nil {
		return
	}

	if j.PruneLHS, err = parsePrunePolicy(asMap.PruneLHS); err != nil {
		err = errors.Wrap(err, "cannot parse 'prune_lhs'")
		return
	}
	if j.PruneRHS, err = parsePrunePolicy(asMap.PruneRHS); err != nil {
		err = errors.Wrap(err, "cannot parse 'prune_rhs'")
		return
	}

	if err = mapstructure.Decode(asMap.Debug, &j.Debug); err != nil {
		err = errors.Wrap(err, "cannot parse 'debug'")
		return
	}

	return
}

func (j *LocalJob) JobName() string {
	return j.Name
}

func (j *LocalJob) JobStart(ctx context.Context) {

	log := ctx.Value(contextKeyLog).(Logger)

	local := rpc.NewLocalRPC()
	handler := Handler{
		Logger: log,
		// Allow access to any dataset since we control what mapping
		// is passed to the pull routine.
		// All local datasets will be passed to its Map() function,
		// but only those for which a mapping exists will actually be pulled.
		// We can pay this small performance penalty for now.
		PullACL: localPullACL{},
	}
	registerEndpoints(local, handler)

	err := doPull(PullContext{local, log, j.Mapping, j.InitialReplPolicy})
	if err != nil {
		log.Printf("error doing pull: %s", err)
	}
}