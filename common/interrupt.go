package common

import (
	"context"
	"os"
	"os/signal"

	"github.com/rs/zerolog/log"
)

func HandleInterruptSignal(cancel context.CancelFunc) {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)

	<-quit
	log.Info().Msg("Interrupt signal sent")
	cancel()
}
