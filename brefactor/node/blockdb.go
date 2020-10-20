package node

import (
	"context"
	"github.com/ZhongAnTech/BlockDB/brefactor/core"
	"github.com/ZhongAnTech/BlockDB/brefactor/plugins/clients/og"
	"github.com/ZhongAnTech/BlockDB/brefactor/plugins/listeners/web"
	"github.com/ZhongAnTech/BlockDB/brefactor/storage"
	"github.com/ZhongAnTech/BlockDB/brefactor/syncer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"time"
)

type BlockDB struct {
	components []core.Component
}

func (n *BlockDB) Start() {
	for _, component := range n.components {
		logrus.Infof("Starting %s", component.Name())
		component.Start()
		logrus.Infof("Started: %s", component.Name())

	}
	logrus.Info("BlockDB engine started")
}

func (n *BlockDB) Stop() {
	//status.Stopped = true
	for i := len(n.components) - 1; i >= 0; i-- {
		component := n.components[i]
		logrus.Infof("Stopping %s", component.Name())
		component.Stop()
		logrus.Infof("Stopped: %s", component.Name())
	}
	logrus.Info("BlockDB engine stopped gracefully")
}

func (n *BlockDB) Name() string {
	return "BlockDB"
}

func (n *BlockDB) InitDefault() {
	n.components = []core.Component{}
}

func (n *BlockDB) Setup() {
	// init components.

	// External data storage facilities. (Dai Yunong)
	// StorageExecutor
	connectionTimeout := time.Millisecond * time.Duration(viper.GetInt("storage.mongodb.timeout_connect_ms"))
	ctx, _ := context.WithTimeout(context.Background(), connectionTimeout)
	storageExecutor, err := storage.Connect(ctx,
		viper.GetString("storage.mongodb.url"),
		viper.GetString("storage.mongodb.database"),
		viper.GetString("storage.mongodb.auth_method"),
		viper.GetString("storage.mongodb.username"),
		viper.GetString("storage.mongodb.password"))
	if err != nil {
		logrus.WithError(err).Fatal("failed to connect to mongodb")
	}

	// will inject the storageExecutor to multiple components.
	businessReader := core.NewBusinessReader(storageExecutor)

	// TODO: RPC server to receive http requests. (Wu Jianhang)
	if viper.GetBool("listener.http.enabled") {
		p := &web.HttpListener{
			JsonCommandParser:       &core.DefaultJsonCommandParser{}, // parse json command
			BlockDBCommandProcessor: &core.DefaultCommandProcessor{},  // send command to ledger
			Config: web.HttpListenerConfig{
				Port:              viper.GetInt("listener.http.port"),
				MaxContentLength:  viper.GetInt64("listener.http.max_content_length"),
				DBActionTimeoutMs: viper.GetInt("listener.http.timeout_db_ms"),
			},
			BusinessReader: businessReader,
		}

		p.Setup()
		n.components = append(n.components, p)
	}

	// TODO: Command Executor (Fang Ning)
	// CommandExecutor

	// TODO: Blockchain sender to send new tx consumed from queue. (Ding Qingyun)
	client := &og.OgClient{
		Config: &og.OgClientConfig{
			LedgerUrl:  viper.GetString("blockchain.og.url"),
			RetryTimes: viper.GetInt("blockchain.og.retry_times"),
		},
		StorageExecutor: storageExecutor,
	}
	client.InitDefault()
	n.components = append(n.components, client)

	// TODO: Sync manager to sync from lastHeight to maxHeight. (Wu Jianhang)
	// LedgerSyncer
	//websocket
	ws := &syncer.WebsocketInfoReceiver{}
	if viper.GetBool("blockchain.og.enable") {
		ws = &syncer.WebsocketInfoReceiver{
			WebsocketUrl: viper.GetString("blockchain.og.wsclient.url"),
			HeightChan: make(chan int64,10),
		}
		ws.Start()
		n.components = append(n.components, ws)
	}


	// TODO: Websocket server to receive new sequencer messages. (Ding Qingyun)
	// HeightSyncer
	if viper.GetBool("blockchain.og.enable") {
		s := &syncer.OgChainSyncer{
			SyncerConfig: syncer.OgChainSyncerConfig{
				LatestHeightUrl: viper.GetString("blockchain.og.url"),
				WebsocketUrl:    viper.GetString("blockchain.og.wsclient.url"),
			},
			StorageExecutor: storageExecutor,
			InfoReceiver:   ws ,
			Quit:           nil,
			MaxSyncedHeight: 122759,
		}
		s.Start()
		n.components = append(n.components, s)
	}



}