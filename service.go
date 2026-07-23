package main

import (
	"context"
	"log"
	"sync"
	"time"
)

// Run starts the incoming and outgoing SMS loops. Blocks until ctx is cancelled.
func Run(ctx context.Context, cfg *Config, modem *Modem, client *SmsClient) {
	incoming := make(chan SmsMessage, 100)

	var wg sync.WaitGroup

	// Goroutine 1: Poll modem for incoming SMS
	wg.Add(1)
	go func() {
		defer wg.Done()
		incomingLoop(ctx, cfg, modem, incoming)
	}()

	// Goroutine 2: Dispatch incoming SMS to server
	wg.Add(1)
	go func() {
		defer wg.Done()
		dispatchLoop(ctx, cfg, modem, client, incoming)
	}()

	// Goroutine 3: Poll server for outgoing SMS and send via modem
	wg.Add(1)
	go func() {
		defer wg.Done()
		outgoingLoop(ctx, cfg, modem, client)
	}()

	<-ctx.Done()
	log.Printf("[INFO] shutting down services")
	wg.Wait()
}

// incomingLoop polls the modem for new SMS and puts them on the channel.
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
				log.Printf("[ERROR] modem read failed: %v", err)
				continue
			}
			for _, msg := range messages {
				select {
				case ch <- msg:
					log.Printf("[INFO] incoming SMS queued from=%s index=%d", msg.Sender, msg.Index)
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// dispatchLoop takes SMS from the channel and reports them to the server, then deletes from modem.
func dispatchLoop(ctx context.Context, cfg *Config, modem *Modem, client *SmsClient, ch <-chan SmsMessage) {
	for {
		select {
		case <-ctx.Done():
			// Drain remaining messages
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
				log.Printf("[WARN] delete from modem failed index=%d err=%v", msg.Index, err)
			}
		}
	}
}

// outgoingLoop polls the server for pending SMS and sends them via modem.
func outgoingLoop(ctx context.Context, cfg *Config, modem *Modem, client *SmsClient) {
	ticker := time.NewTicker(time.Duration(cfg.Sms.OutgoingPollMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sms, err := client.PollOutgoing()
			if err != nil {
				log.Printf("[ERROR] poll outgoing failed: %v", err)
				continue
			}
			if sms == nil {
				continue
			}

			log.Printf("[INFO] sending SMS id=%s to=%s", sms.ID, sms.Numb)
			if err := modem.SendSms(sms.Numb, sms.Text); err != nil {
				log.Printf("[ERROR] send failed id=%s err=%v", sms.ID, err)
				if cErr := client.ReportFail(sms.ID, err.Error()); cErr != nil {
					log.Printf("[ERROR] report fail failed: %v", cErr)
				}
			} else {
				if cErr := client.ConfirmSent(sms.ID); cErr != nil {
					log.Printf("[ERROR] confirm sent failed: %v", cErr)
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
			log.Printf("[INFO] incoming reported from=%s", msg.Sender)
			return
		}
		log.Printf("[WARN] report incoming failed attempt=%d err=%v", attempt, err)
		if attempt <= maxRetries {
			time.Sleep(delay)
		}
	}
	log.Printf("[ERROR] report incoming gave up from=%s", msg.Sender)
}
