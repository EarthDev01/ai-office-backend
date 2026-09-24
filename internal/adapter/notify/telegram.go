// Package notify = แจ้งเตือนโควตา/contract test ผ่าน Telegram (ห้องมาจาก settings.telegram_rooms · OQ-24)
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Telegram struct {
	token string // ██ secret ห้าม log
	http  *http.Client
}

func NewTelegram(token string) *Telegram {
	return &Telegram{token: token, http: &http.Client{Timeout: 5 * time.Second}}
}

// Notify · ไม่ได้ตั้ง token หรือห้อง = เขียน log แทน (ไม่ทำให้งานหลักล้ม)
func (t *Telegram) Notify(ctx context.Context, room, text string) error {
	if t.token == "" || room == "" {
		log.Printf("[NOTIFY] %s", text)
		return nil
	}
	body, _ := json.Marshal(map[string]any{"chat_id": room, "text": text, "disable_web_page_preview": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+t.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.http.Do(req)
	if err != nil {
		log.Printf("[NOTIFY] ส่ง Telegram ไม่สำเร็จ: %v", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram ตอบ %d", resp.StatusCode)
	}
	return nil
}
