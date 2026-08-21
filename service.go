package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

func Run(ctx context.Context, cfg *Config, modem *Modem, client *SmsClient) {
	incoming := make(chan SmsMessage, 100)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		incomingLoop(ctx, cfg, modem, incoming)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		dispatchLoop(ctx, cfg, modem, client, incoming)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		outgoingLoop(ctx, cfg, modem, client)
	}()

	<-ctx.Done()
	slog.Info("shutting down services")
	wg.Wait()
}

func incomingLoop(ctx context.Context, cfg *Config, modem *Modem, ch chan<- SmsMessage) {
	ticker := time.NewTicker(time.Duration(cfg.Sms.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages, err := modem.ReadAll()
			if err != nil {
				slog.Error("modem read failed", "err", err)
				continue
			}
			for _, msg := range messages {
				select {
				case ch <- msg:
					slog.Info("incoming SMS queued", "from", msg.Sender, "index", msg.Index)
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func dispatchLoop(ctx context.Context, cfg *Config, modem *Modem, client *SmsClient, ch <-chan SmsMessage) {
	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case msg := <-ch:
					reportWithRetry(cfg, client, msg)
					modem.Delete(msg.Index)
				default:
					return
				}
			}
		case msg := <-ch:
			reportWithRetry(cfg, client, msg)
			if err := modem.Delete(msg.Index); err != nil {
				slog.Warn("delete from modem failed", "index", msg.Index, "err", err)
			}
		}
	}
}

func outgoingLoop(ctx context.Context, cfg *Config, modem *Modem, client *SmsClient) {
	ticker := time.NewTicker(time.Duration(cfg.Sms.OutgoingPollMs) * time.Millisecond)
	defer ticker.Stop()

	heartbeat := time.NewTicker(60 * time.Second)
	defer heartbeat.Stop()
	pollCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			slog.Info("heartbeat", "polls", pollCount)
			pollCount = 0
		case <-ticker.C:
			pollCount++
			sms, err := client.PollOutgoing()
			if err != nil {
				slog.Error("poll outgoing failed", "err", err)
				continue
			}
			if sms == nil {
				continue
			}

			slog.Info("sending SMS", "id", sms.ID, "to", sms.Numb)
			if err := modem.SendSms(sms.Numb, sms.Text); err != nil {
				slog.Error("send failed", "id", sms.ID, "err", err)
				if cErr := client.ReportFail(sms.ID, err.Error()); cErr != nil {
					slog.Error("report fail failed", "err", cErr)
				}
			} else {
				if cErr := client.ConfirmSent(sms.ID); cErr != nil {
					slog.Error("confirm sent failed", "err", cErr)
				}
			}
		}
	}
}

func reportWithRetry(cfg *Config, client *SmsClient, msg SmsMessage) {
	maxRetries := cfg.Sms.SendMaxRetries
	delay := time.Duration(cfg.Sms.SendRetryDelayMs) * time.Millisecond

	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		err := client.ReportIncoming(msg.Sender, msg.Text, msg.Timestamp)
		if err == nil {
			slog.Info("incoming reported", "from", msg.Sender)
			return
		}
		slog.Warn("report incoming failed", "attempt", attempt, "err", err)
		if attempt <= maxRetries {
			time.Sleep(delay)
		}
	}
	slog.Error("report incoming gave up", "from", msg.Sender)
}
