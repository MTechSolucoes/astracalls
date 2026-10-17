package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// 3CX: o tronco registra com o número interno dele no From/To (ex.: 96016) e o
// usuário SIP da sessão só no Authorization. O registro tem que casar pela
// credencial do Digest e o INVITE de volta pelo IP do PBX registrado.
func TestSIPRegisterByDigestUsername(t *testing.T) {
	log := slog.Default()
	mgr := &SessionManager{broker: NewBroker(), log: log, maxCalls: 8, sessions: map[string]*Session{}}
	sess := &Session{id: "s1", mgr: mgr, log: log, reg: newCallRegistry(), SIPUser: "wa_test1234", SIPPass: "segredo"}
	mgr.register(sess)

	gw, err := NewSIPGateway(mgr, log)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = gw.server.ListenAndServe(ctx, "udp", "127.0.0.1:15093") }()

	ua, _ := sipgo.NewUA(sipgo.WithUserAgent("3CX-test"))
	cli, _ := sipgo.NewClient(ua, sipgo.WithClientHostname("127.0.0.1"), sipgo.WithClientPort(15094))
	// o PBX escuta na porta do Via para receber as respostas por UDP
	pbx, _ := sipgo.NewServer(ua)
	go func() { _ = pbx.ListenAndServe(ctx, "udp", "127.0.0.1:15094") }()
	time.Sleep(200 * time.Millisecond)

	target := sip.Uri{Scheme: "sip", Host: "127.0.0.1", Port: 15093}
	register := func(auth string) *sip.Response {
		t.Helper()
		req := sip.NewRequest(sip.REGISTER, target)
		aor := sip.Uri{Scheme: "sip", User: "96016", Host: "127.0.0.1"}
		from := &sip.FromHeader{Address: aor, Params: sip.NewParams()}
		from.Params.Add("tag", "t1")
		req.AppendHeader(from)
		req.AppendHeader(&sip.ToHeader{Address: aor, Params: sip.NewParams()})
		req.AppendHeader(&sip.ContactHeader{Address: sip.Uri{Scheme: "sip", User: "96016", Host: "127.0.0.1", Port: 15094}})
		req.AppendHeader(sip.NewHeader("Expires", "120"))
		if auth != "" {
			req.AppendHeader(sip.NewHeader("Authorization", auth))
		}
		rctx, rcancel := context.WithTimeout(ctx, 3*time.Second)
		defer rcancel()
		res, err := cli.Do(rctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	// 1º REGISTER sem credencial e com From desconhecido: desafia (não 403).
	if res := register(""); res.StatusCode != 401 {
		t.Fatalf("esperava 401, got %d", res.StatusCode)
	}
	// Credencial de outro usuário: 403.
	if res := register(buildAuth("wa_outro", "astracalls", "segredo", "REGISTER", "sip:127.0.0.1:15093", "n1")); res.StatusCode != 403 {
		t.Fatalf("esperava 403, got %d", res.StatusCode)
	}
	// Senha errada: novo desafio.
	if res := register(buildAuth("wa_test1234", "astracalls", "errada", "REGISTER", "sip:127.0.0.1:15093", "n1")); res.StatusCode != 401 {
		t.Fatalf("esperava 401, got %d", res.StatusCode)
	}
	// Credencial certa no Digest: registra o From (96016) apontando para a sessão.
	if res := register(buildAuth("wa_test1234", "astracalls", "segredo", "REGISTER", "sip:127.0.0.1:15093", "n1")); res.StatusCode != 200 {
		t.Fatalf("esperava 200, got %d", res.StatusCode)
	}
	gw.mu.RLock()
	reg, ok := gw.registrations["96016"]
	gw.mu.RUnlock()
	if !ok || reg.sessionID != "s1" {
		t.Fatalf("registro não casou com a sessão: %+v", reg)
	}

	// INVITE do 3CX com o número do chamador no From: casa pelo IP do PBX.
	if s := gw.sessionForRegisteredPBX("127.0.0.1"); s == nil || s.id != "s1" {
		t.Fatalf("sessionForRegisteredPBX = %v", s)
	}
	if s := gw.sessionForRegisteredPBX("10.9.9.9"); s != nil {
		t.Fatalf("IP desconhecido casou com %s", s.id)
	}
}
