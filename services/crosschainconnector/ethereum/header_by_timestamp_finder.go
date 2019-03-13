package ethereum

import (
	"context"
	"fmt"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/pkg/errors"
	"math"
	"math/big"
	"time"
)

type TimestampFetcher interface {
	GetBlockByTimestamp(ctx context.Context, nano primitives.TimestampNano) (*big.Int, error)
}

type finder struct {
	logger    log.BasicLogger
	btg       BlockAndTimestampGetter
	lastKnown *BlockHeightAndTime
}

func NewTimestampFetcher(btg BlockAndTimestampGetter, logger log.BasicLogger) *finder {
	f := &finder{
		btg:    btg,
		logger: logger,
	}

	return f
}

func (f *finder) GetBlockByTimestamp(ctx context.Context, nano primitives.TimestampNano) (*big.Int, error) {
	timestampInSeconds := int64(nano) / int64(time.Second)
	// ethereum started around 2015/07/31
	if timestampInSeconds < 1438300800 {
		return nil, errors.New("cannot query before ethereum genesis")
	}

	// approx always returns a new pointer
	latest, err := f.btg.ApproximateBlockAt(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest block")
	}

	if latest == nil { // simulator always returns nil block number
		return nil, nil
	}

	f.lastKnown = latest

	requestedTime := time.Unix(0, int64(nano))
	latestTime := time.Unix(0, latest.TimeSeconds*int64(time.Second))
	truncatedRequestedTime := requestedTime.Add(time.Duration(-requestedTime.Nanosecond()))
	if latestTime.Before(truncatedRequestedTime) {
		return nil, errors.Errorf("requested future block at time %s, latest block time is %s", requestedTime.UTC(), latestTime.UTC())
	}

	if latest.TimeSeconds == truncatedRequestedTime.Unix() {
		return big.NewInt(latest.Number), nil
	}

	latestNum := latest.Number - 10000
	// this was added to support simulations and tests, should not be relevant for real eth as there are more than 10k blocks there
	if latestNum < 0 {
		latestNum = 0
	}
	back10k, err := f.btg.ApproximateBlockAt(ctx, big.NewInt(latestNum))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get past reference block")
	}

	theBlock, err := f.findBlockByTimeStamp(ctx, timestampInSeconds, back10k, latest)
	return theBlock, err
}

func (f *finder) findBlockByTimeStamp(ctx context.Context, targetTimestamp int64, current, prev *BlockHeightAndTime) (*big.Int, error) {
	f.logger.Info("searching for block in ethereum",
		log.Int64("target-timestamp", targetTimestamp),
		log.Int64("current-block-number", current.Number),
		log.Int64("current-timestamp", current.TimeSeconds),
		log.Int64("prev-block-number", prev.Number),
		log.Int64("prev-timestamp", prev.TimeSeconds))
	blockNumberDiff := current.Number - prev.Number

	// we stop when the range we are in-between is 1 or 0 (same block), it means we found a block with the exact timestamp or closest from below
	if blockNumberDiff == 1 || blockNumberDiff == 0 {
		// if the block we are returning has a ts > target, it means we want one block before (so our ts is always bigger than block ts)
		if current.TimeSeconds > targetTimestamp {
			return big.NewInt(current.Number - 1), nil
		} else {
			return big.NewInt(current.Number), nil
		}
	}

	timeDiff := current.TimeSeconds - prev.TimeSeconds
	if timeDiff == 0 {
		return nil, errors.Errorf("two blocks with the same timestamp detected - invalid state (ganache?) b1: %+v, b2: %+v", current, prev)
	}
	secondsPerBlock := int64(math.Ceil(float64(timeDiff) / float64(blockNumberDiff)))
	distanceToTargetFromCurrent := current.TimeSeconds - targetTimestamp
	blocksToJump := distanceToTargetFromCurrent / secondsPerBlock
	f.logger.Info("eth block search delta", log.Int64("jump-backwards", blocksToJump))
	guessBlockNumber := current.Number - blocksToJump

	// this will handle the case where we 'went' too far due to uneven distribution of time differences between blocks
	if guessBlockNumber > f.lastKnown.Number {
		return f.findBlockByTimeStamp(ctx, targetTimestamp, f.lastKnown, current)
	}

	guess, err := f.btg.ApproximateBlockAt(ctx, big.NewInt(guessBlockNumber))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get header by block number %d", guessBlockNumber))
	}

	return f.findBlockByTimeStamp(ctx, targetTimestamp, guess, current)
}
