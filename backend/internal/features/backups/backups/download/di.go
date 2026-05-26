package backups_download

import (
	"databasus-backend/internal/config"
	"databasus-backend/internal/util/logger"
)

var downloadTokenRepository = &DownloadTokenRepository{}

var downloadTracker = NewDownloadTracker()

var (
	bandwidthManager               *BandwidthManager
	downloadTokenService           *DownloadTokenService
	downloadTokenBackgroundService *DownloadTokenBackgroundService
)

func init() {
	env := config.GetEnv()
	throughputMBs := env.NodeNetworkThroughputMBs
	if throughputMBs == 0 {
		throughputMBs = 125
	}
	bandwidthManager = NewBandwidthManager(throughputMBs)

	downloadTokenService = &DownloadTokenService{
		downloadTokenRepository,
		logger.GetLogger(),
		downloadTracker,
		bandwidthManager,
	}

	downloadTokenBackgroundService = &DownloadTokenBackgroundService{
		downloadTokenService: downloadTokenService,
		logger:               logger.GetLogger(),
	}
}

func GetDownloadTokenService() *DownloadTokenService {
	return downloadTokenService
}

func GetDownloadTokenBackgroundService() *DownloadTokenBackgroundService {
	return downloadTokenBackgroundService
}

func GetBandwidthManager() *BandwidthManager {
	return bandwidthManager
}
