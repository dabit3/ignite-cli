package cosmosclient

import (
	"context"
	"testing"
	"time"

	"github.com/ignite-hq/cli/ignite/pkg/cosmosclient/testutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	rpcmocks "github.com/tendermint/tendermint/rpc/client/mocks"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestGetBlockTXs(t *testing.T) {
	m := rpcmocks.Client{}
	m.Test(t)

	// Mock the Block RPC endpoint
	ctx := context.Background()
	block := createTestBlock(1)

	m.On("Block", ctx, &block.Height).Return(&ctypes.ResultBlock{Block: &block}, nil)

	// Mock the TxSearch RPC endpoint
	searchQry := createTxSearchByHeightQuery(block.Height)
	page := 1
	perPage := defaultTXsPerPage
	rtx := ctypes.ResultTx{}
	resSearch := ctypes.ResultTxSearch{
		Txs:        []*ctypes.ResultTx{&rtx},
		TotalCount: 1,
	}

	m.On("TxSearch", ctx, searchQry, false, &page, &perPage, orderAsc).Return(&resSearch, nil)

	// Create a cosmos client with an RPC mock
	client := Client{RPC: &m}

	txs, err := client.GetBlockTXs(ctx, block.Height)
	require.NoError(t, err)
	require.Equal(t, txs, []TX{
		{
			BlockTime: block.Time,
			Raw:       &rtx,
		},
	})

	m.AssertNumberOfCalls(t, "Block", 1)
	m.AssertNumberOfCalls(t, "TxSearch", 1)
}

func TestGetBlockTXsWithBlockError(t *testing.T) {
	m := rpcmocks.Client{}
	m.Test(t)

	wantErr := errors.New("expected error")

	// Mock the Block RPC endpoint
	ctx := context.Background()
	height := int64(1)

	m.On("Block", ctx, &height).Return(nil, wantErr)

	// Create a cosmos client with an RPC mock
	client := Client{RPC: &m}

	txs, err := client.GetBlockTXs(ctx, height)
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, txs)

	m.AssertNumberOfCalls(t, "Block", 1)
	m.AssertNumberOfCalls(t, "TxSearch", 0)
}

func TestGetBlockTXsPagination(t *testing.T) {
	m := rpcmocks.Client{}
	m.Test(t)

	// Mock the Block RPC endpoint
	ctx := context.Background()
	block := createTestBlock(1)

	m.On("Block", ctx, &block.Height).Return(&ctypes.ResultBlock{Block: &block}, nil)

	// Mock the TxSearch RPC endpoint and fake the number of
	// transactions so it is called twice to fetch two pages
	searchQry := createTxSearchByHeightQuery(block.Height)
	perPage := defaultTXsPerPage
	fakeCount := perPage + 1
	first := 1
	second := 2
	firstPage := ctypes.ResultTxSearch{
		Txs:        []*ctypes.ResultTx{{}},
		TotalCount: fakeCount,
	}
	secondPage := ctypes.ResultTxSearch{
		Txs:        []*ctypes.ResultTx{{}},
		TotalCount: fakeCount,
	}

	m.On("TxSearch", ctx, searchQry, false, &first, &perPage, orderAsc).Return(&firstPage, nil)
	m.On("TxSearch", ctx, searchQry, false, &second, &perPage, orderAsc).Return(&secondPage, nil)

	// Create a cosmos client with an RPC mock
	client := Client{RPC: &m}

	txs, err := client.GetBlockTXs(ctx, block.Height)
	require.NoError(t, err)
	require.Equal(t, txs, []TX{
		{
			BlockTime: block.Time,
			Raw:       firstPage.Txs[0],
		},
		{
			BlockTime: block.Time,
			Raw:       secondPage.Txs[0],
		},
	})

	m.AssertNumberOfCalls(t, "Block", 1)
	m.AssertNumberOfCalls(t, "TxSearch", 2)
}

func TestGetBlockTXsWithSearchError(t *testing.T) {
	m := rpcmocks.Client{}
	m.Test(t)

	wantErr := errors.New("expected error")

	// Mock the Block RPC endpoint
	ctx := context.Background()
	block := createTestBlock(1)

	m.On("Block", ctx, &block.Height).Return(&ctypes.ResultBlock{Block: &block}, nil)

	// Mock the TxSearch RPC endpoint
	searchQry := createTxSearchByHeightQuery(block.Height)
	perPage := defaultTXsPerPage
	page := 1

	m.On("TxSearch", ctx, searchQry, false, &page, &perPage, orderAsc).Return(nil, wantErr)

	// Create a cosmos client with an RPC mock
	client := Client{RPC: &m}

	txs, err := client.GetBlockTXs(ctx, block.Height)
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, txs)

	m.AssertNumberOfCalls(t, "Block", 1)
	m.AssertNumberOfCalls(t, "TxSearch", 1)
}

func TestCollectTXs(t *testing.T) {
	m := testutil.NewTendermintClientMock(t)

	// Mock the Status RPC endpoint to report that only two blocks exists
	status := ctypes.ResultStatus{
		SyncInfo: ctypes.SyncInfo{
			LatestBlockHeight: 2,
		},
	}

	m.OnStatus().Return(&status, nil)

	// Mock the Block RPC endpoint to return two blocks
	b1 := createTestBlock(1)
	b2 := createTestBlock(2)

	m.On("Block", mock.Anything, &b1.Height).Return(&ctypes.ResultBlock{Block: &b1}, nil)
	m.On("Block", mock.Anything, &b2.Height).Return(&ctypes.ResultBlock{Block: &b2}, nil)

	// Mock the TxSearch RPC endpoint to return each of the two block.
	// Transactions are empty because only the pointer address is required to assert.
	page := 1
	perPage := defaultTXsPerPage
	q1 := createTxSearchByHeightQuery(1)
	r1 := ctypes.ResultTxSearch{
		Txs:        []*ctypes.ResultTx{{}},
		TotalCount: 1,
	}
	q2 := createTxSearchByHeightQuery(2)
	r2 := ctypes.ResultTxSearch{
		Txs:        []*ctypes.ResultTx{{}, {}},
		TotalCount: 2,
	}

	m.On("TxSearch", mock.Anything, q1, false, &page, &perPage, orderAsc).Return(&r1, nil)
	m.On("TxSearch", mock.Anything, q2, false, &page, &perPage, orderAsc).Return(&r2, nil)

	// Prepare expected values
	wantTXs := []TX{
		{
			BlockTime: b1.Time,
			Raw:       r1.Txs[0],
		},
		{
			BlockTime: b2.Time,
			Raw:       r2.Txs[0],
		},
		{
			BlockTime: b2.Time,
			Raw:       r2.Txs[1],
		},
	}

	// Create a cosmos client that uses the RPC mock
	client := Client{RPC: m}

	// Create a channel to receive the transactions from the two blocks.
	// The channel must be closed after the call to collect.
	tc := make(chan []TX)

	// Collect all transactions
	var (
		txs  []TX
		open bool
	)

	finished := make(chan struct{})
	go func() {
		defer close(finished)

		for t := range tc {
			txs = append(txs, t...)
		}
	}()

	err := client.CollectTXs(context.Background(), 1, tc)

	select {
	case <-time.After(time.Second):
		t.Fatal("expected CollectTXs to finish sooner")
	case <-finished:
	}

	select {
	case _, open = <-tc:
	default:
	}

	// Assert
	require.NoError(t, err)
	require.Equal(t, wantTXs, txs)
	require.False(t, open, "expected transaction channel to be closed")
}

func TestCollectTXsWithStatusError(t *testing.T) {
	m := testutil.NewTendermintClientMock(t)

	wantErr := errors.New("expected error")

	// Mock the Status RPC endpoint to return an error
	m.OnStatus().Return(nil, wantErr)

	// Create a cosmos client that uses the RPC mock
	client := Client{RPC: m}

	// Create a channel to receive the transactions from the two blocks.
	// The channel must be closed after the call to collect.
	tc := make(chan []TX)

	open := false
	ctx := context.Background()
	err := client.CollectTXs(ctx, 1, tc)

	select {
	case _, open = <-tc:
	default:
	}

	// Assert
	require.ErrorIs(t, err, wantErr)
	require.False(t, open, "expected transaction channel to be closed")
}

func TestCollectTXsWithBlockError(t *testing.T) {
	m := testutil.NewTendermintClientMock(t)

	wantErr := errors.New("expected error")

	// Mock the Status RPC endpoint
	status := ctypes.ResultStatus{
		SyncInfo: ctypes.SyncInfo{
			LatestBlockHeight: 1,
		},
	}

	m.OnStatus().Return(&status, nil)

	// Mock the Block RPC endpoint to return an error
	m.OnBlock().Return(nil, wantErr)

	// Create a cosmos client that uses the RPC mock
	client := Client{RPC: m}

	// Create a channel to receive the transactions from the two blocks.
	// The channel must be closed after the call to collect.
	tc := make(chan []TX)

	open := false
	ctx := context.Background()
	err := client.CollectTXs(ctx, 1, tc)

	select {
	case _, open = <-tc:
	default:
	}

	// Assert
	require.ErrorIs(t, err, wantErr)
	require.False(t, open, "expected transaction channel to be closed")
}

func TestCollectTXsWithContextDone(t *testing.T) {
	m := testutil.NewTendermintClientMock(t)

	// Mock the Status RPC endpoint
	status := ctypes.ResultStatus{
		SyncInfo: ctypes.SyncInfo{
			LatestBlockHeight: 1,
		},
	}

	m.OnStatus().Return(&status, nil)

	// Mock the Block RPC endpoint
	block := createTestBlock(1)

	m.OnBlock().Return(&ctypes.ResultBlock{Block: &block}, nil)

	// Mock the TxSearch RPC endpoint
	rs := ctypes.ResultTxSearch{
		Txs:        []*ctypes.ResultTx{{}},
		TotalCount: 1,
	}

	m.OnTxSearch().Return(&rs, nil)

	// Create a cosmos client that uses the RPC mock
	client := Client{RPC: m}

	// Create a channel to receive the transactions from the two blocks.
	// The channel must be closed after the call to collect.
	tc := make(chan []TX)

	// Create a context and cancel it so the collect call finishes because the context is done
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	open := false
	err := client.CollectTXs(ctx, 1, tc)

	select {
	case _, open = <-tc:
	default:
	}

	// Assert
	require.ErrorIs(t, err, ctx.Err())
	require.False(t, open, "expected transaction channel to be closed")
}

func createTestBlock(height int64) tmtypes.Block {
	return tmtypes.Block{
		Header: tmtypes.Header{
			Height: height,
			Time:   time.Now(),
		},
	}
}