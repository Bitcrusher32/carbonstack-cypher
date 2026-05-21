package main

import (
"bytes"
"encoding/base64"
"encoding/json"
"errors"
"flag"
"fmt"
"io"
"net/http"
"os"
"path/filepath"
"strings"
"time"
)

const (
defaultServerURL = "http://localhost:8080"
defaultStatePath = ".carbonstack-comms/state.json"

contentTypeTextStub = "carbonstack.message.text.stub.v0"
protocolVersionStub = "stub-v0"
)

type State struct {
ServerURL           string `json:"server_url"`
AccountID           string `json:"account_id"`
DisplayName         string `json:"display_name"`
DeviceID            string `json:"device_id"`
DeviceLabel         string `json:"device_label"`
PublicIdentityKey   string `json:"public_identity_key"`
PublicPrekeyBundle  string `json:"public_prekey_bundle"`
ProtocolVersion     string `json:"protocol_version"`
}

type ErrorResponse struct {
Error struct {
Code    string `json:"code"`
Message string `json:"message"`
} `json:"error"`
}

func main() {
if len(os.Args) < 2 {
usage()
os.Exit(1)
}

var err error

switch os.Args[1] {
case "init":
err = cmdInit(os.Args[2:])
case "dev-create-invite":
err = cmdDevCreateInvite(os.Args[2:])
case "claim-invite":
err = cmdClaimInvite(os.Args[2:])
case "register-device":
err = cmdRegisterDevice(os.Args[2:])
case "list-devices":
err = cmdListDevices(os.Args[2:])
case "send":
err = cmdSend(os.Args[2:])
case "inbox":
err = cmdInbox(os.Args[2:])
case "ack":
err = cmdAck(os.Args[2:])
default:
err = fmt.Errorf("unknown command: %s", os.Args[1])
}

if err != nil {
fmt.Fprintf(os.Stderr, "error: %v\n", err)
os.Exit(1)
}
}

func usage() {
fmt.Println("CarbonStackComms Phase 1 CLI")
fmt.Println()
fmt.Println("Commands:")
fmt.Println("  init")
fmt.Println("  dev-create-invite")
fmt.Println("  claim-invite")
fmt.Println("  register-device")
fmt.Println("  list-devices")
fmt.Println("  send")
fmt.Println("  inbox")
fmt.Println("  ack")
fmt.Println()
fmt.Println("Example:")
fmt.Println("  go run ./cmd/comms init --state .alice/state.json --server http://localhost:8080")
}

func cmdInit(args []string) error {
fs := flag.NewFlagSet("init", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
serverURL := fs.String("server", defaultServerURL, "CarbonStackCypher server URL")
if err := fs.Parse(args); err != nil {
return err
}

state := State{
ServerURL:       strings.TrimRight(*serverURL, "/"),
ProtocolVersion: protocolVersionStub,
}

if err := saveState(*statePath, state); err != nil {
return err
}

fmt.Printf("initialized state: %s\n", *statePath)
fmt.Printf("server: %s\n", state.ServerURL)
return nil
}

func cmdDevCreateInvite(args []string) error {
fs := flag.NewFlagSet("dev-create-invite", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
serverURL := fs.String("server", "", "CarbonStackCypher server URL override")
inviteCode := fs.String("invite", "", "invite code to create")
if err := fs.Parse(args); err != nil {
return err
}

server, err := serverFromStateOrFlag(*statePath, *serverURL)
if err != nil {
return err
}

req := map[string]string{
"invite_code": *inviteCode,
}

var resp map[string]string
if err := postJSON(server+"/v0/dev/invites", req, &resp); err != nil {
return err
}

fmt.Println("dev invite created")
fmt.Printf("invite_id: %s\n", resp["invite_id"])
fmt.Printf("invite_code: %s\n", resp["invite_code"])
fmt.Printf("created_at: %s\n", resp["created_at"])
return nil
}

func cmdClaimInvite(args []string) error {
fs := flag.NewFlagSet("claim-invite", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
serverURL := fs.String("server", "", "CarbonStackCypher server URL override")
inviteCode := fs.String("invite", "", "invite code")
displayName := fs.String("name", "", "display name")
if err := fs.Parse(args); err != nil {
return err
}

if *inviteCode == "" || *displayName == "" {
return errors.New("--invite and --name are required")
}

state, _ := loadState(*statePath)
server := strings.TrimRight(*serverURL, "/")
if server == "" {
if state.ServerURL != "" {
server = state.ServerURL
} else {
server = defaultServerURL
}
}

req := map[string]string{
"invite_code":  *inviteCode,
"display_name": *displayName,
}

var resp struct {
AccountID string `json:"account_id"`
CreatedAt string `json:"created_at"`
}

if err := postJSON(server+"/v0/invites/claim", req, &resp); err != nil {
return err
}

state.ServerURL = server
state.AccountID = resp.AccountID
state.DisplayName = *displayName
state.ProtocolVersion = protocolVersionStub

if err := saveState(*statePath, state); err != nil {
return err
}

fmt.Println("invite claimed")
fmt.Printf("account_id: %s\n", resp.AccountID)
fmt.Printf("created_at: %s\n", resp.CreatedAt)
return nil
}

func cmdRegisterDevice(args []string) error {
fs := flag.NewFlagSet("register-device", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
label := fs.String("label", "", "device label")
if err := fs.Parse(args); err != nil {
return err
}

if *label == "" {
return errors.New("--label is required")
}

state, err := requireState(*statePath)
if err != nil {
return err
}

if state.AccountID == "" {
return errors.New("state has no account_id; run claim-invite first")
}

publicIdentityKey := "stub-public-identity-key-" + sanitizeLabel(*label)
publicPrekeyBundle := "stub-prekey-bundle-" + sanitizeLabel(*label)

req := map[string]string{
"account_id":            state.AccountID,
"device_label":          *label,
"public_identity_key":   publicIdentityKey,
"public_prekey_bundle":  publicPrekeyBundle,
}

var resp struct {
DeviceID string `json:"device_id"`
AccountID string `json:"account_id"`
CreatedAt string `json:"created_at"`
}

if err := postJSON(state.ServerURL+"/v0/devices/register", req, &resp); err != nil {
return err
}

state.DeviceID = resp.DeviceID
state.DeviceLabel = *label
state.PublicIdentityKey = publicIdentityKey
state.PublicPrekeyBundle = publicPrekeyBundle
state.ProtocolVersion = protocolVersionStub

if err := saveState(*statePath, state); err != nil {
return err
}

fmt.Println("device registered")
fmt.Printf("device_id: %s\n", resp.DeviceID)
fmt.Printf("account_id: %s\n", resp.AccountID)
fmt.Printf("created_at: %s\n", resp.CreatedAt)
return nil
}

func cmdListDevices(args []string) error {
fs := flag.NewFlagSet("list-devices", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
accountID := fs.String("account", "", "account ID to list")
if err := fs.Parse(args); err != nil {
return err
}

state, err := requireState(*statePath)
if err != nil {
return err
}

targetAccountID := *accountID
if targetAccountID == "" {
targetAccountID = state.AccountID
}
if targetAccountID == "" {
return errors.New("no account specified and state has no account_id")
}

var resp struct {
AccountID string `json:"account_id"`
Devices []struct {
DeviceID string `json:"device_id"`
DeviceLabel string `json:"device_label"`
PublicIdentityKey string `json:"public_identity_key"`
PublicPrekeyBundle string `json:"public_prekey_bundle"`
CreatedAt string `json:"created_at"`
} `json:"devices"`
}

if err := getJSON(state.ServerURL+"/v0/accounts/"+targetAccountID+"/devices", &resp); err != nil {
return err
}

fmt.Printf("account_id: %s\n", resp.AccountID)
for _, d := range resp.Devices {
fmt.Println()
fmt.Printf("device_id: %s\n", d.DeviceID)
fmt.Printf("label: %s\n", d.DeviceLabel)
fmt.Printf("public_identity_key: %s\n", d.PublicIdentityKey)
fmt.Printf("created_at: %s\n", d.CreatedAt)
}
return nil
}

func cmdSend(args []string) error {
fs := flag.NewFlagSet("send", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
toDevice := fs.String("to-device", "", "recipient device ID")
message := fs.String("message", "", "message text")
if err := fs.Parse(args); err != nil {
return err
}

if *toDevice == "" || *message == "" {
return errors.New("--to-device and --message are required")
}

state, err := requireReadyDevice(*statePath)
if err != nil {
return err
}

ciphertextB64 := base64.StdEncoding.EncodeToString([]byte(*message))

req := map[string]string{
"sender_device_id":    state.DeviceID,
"recipient_device_id": *toDevice,
"content_type":        contentTypeTextStub,
"protocol_version":    protocolVersionStub,
"ciphertext_b64":      ciphertextB64,
"client_created_at":   time.Now().UTC().Format(time.RFC3339),
}

var resp struct {
EnvelopeID string `json:"envelope_id"`
DeliveryState string `json:"delivery_state"`
ServerReceivedAt string `json:"server_received_at"`
}

if err := postJSON(state.ServerURL+"/v0/envelopes", req, &resp); err != nil {
return err
}

fmt.Println("envelope sent")
fmt.Printf("envelope_id: %s\n", resp.EnvelopeID)
fmt.Printf("delivery_state: %s\n", resp.DeliveryState)
fmt.Printf("server_received_at: %s\n", resp.ServerReceivedAt)
return nil
}

func cmdInbox(args []string) error {
fs := flag.NewFlagSet("inbox", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
if err := fs.Parse(args); err != nil {
return err
}

state, err := requireReadyDevice(*statePath)
if err != nil {
return err
}

var resp struct {
DeviceID string `json:"device_id"`
Envelopes []struct {
EnvelopeID string `json:"envelope_id"`
SenderDeviceID string `json:"sender_device_id"`
RecipientDeviceID string `json:"recipient_device_id"`
ContentType string `json:"content_type"`
ProtocolVersion string `json:"protocol_version"`
CiphertextB64 string `json:"ciphertext_b64"`
ClientCreatedAt string `json:"client_created_at"`
ServerReceivedAt string `json:"server_received_at"`
DeliveryState string `json:"delivery_state"`
} `json:"envelopes"`
}

if err := getJSON(state.ServerURL+"/v0/devices/"+state.DeviceID+"/envelopes", &resp); err != nil {
return err
}

fmt.Printf("device_id: %s\n", resp.DeviceID)
fmt.Printf("queued_envelopes: %d\n", len(resp.Envelopes))

for _, e := range resp.Envelopes {
decoded, err := base64.StdEncoding.DecodeString(e.CiphertextB64)
body := ""
if err == nil {
body = string(decoded)
} else {
body = "[invalid stub ciphertext]"
}

fmt.Println()
fmt.Printf("envelope_id: %s\n", e.EnvelopeID)
fmt.Printf("from_device: %s\n", e.SenderDeviceID)
fmt.Printf("state: %s\n", e.DeliveryState)
fmt.Printf("server_received_at: %s\n", e.ServerReceivedAt)
fmt.Printf("stub_plaintext: %s\n", body)
}

return nil
}

func cmdAck(args []string) error {
fs := flag.NewFlagSet("ack", flag.ExitOnError)
statePath := fs.String("state", defaultStatePath, "local state file path")
envelopeID := fs.String("envelope", "", "envelope ID to acknowledge")
if err := fs.Parse(args); err != nil {
return err
}

if *envelopeID == "" {
return errors.New("--envelope is required")
}

state, err := requireReadyDevice(*statePath)
if err != nil {
return err
}

req := map[string]string{
"recipient_device_id": state.DeviceID,
}

var resp struct {
EnvelopeID string `json:"envelope_id"`
DeliveryState string `json:"delivery_state"`
AcknowledgedAt string `json:"acknowledged_at"`
}

if err := postJSON(state.ServerURL+"/v0/envelopes/"+*envelopeID+"/ack", req, &resp); err != nil {
return err
}

fmt.Println("envelope acknowledged")
fmt.Printf("envelope_id: %s\n", resp.EnvelopeID)
fmt.Printf("delivery_state: %s\n", resp.DeliveryState)
fmt.Printf("acknowledged_at: %s\n", resp.AcknowledgedAt)
return nil
}

func loadState(path string) (State, error) {
var state State

body, err := os.ReadFile(path)
if err != nil {
return state, err
}

if err := json.Unmarshal(body, &state); err != nil {
return state, err
}

return state, nil
}

func requireState(path string) (State, error) {
state, err := loadState(path)
if err != nil {
return State{}, fmt.Errorf("load state %s: %w", path, err)
}
if state.ServerURL == "" {
state.ServerURL = defaultServerURL
}
state.ServerURL = strings.TrimRight(state.ServerURL, "/")
return state, nil
}

func requireReadyDevice(path string) (State, error) {
state, err := requireState(path)
if err != nil {
return State{}, err
}
if state.DeviceID == "" {
return State{}, errors.New("state has no device_id; run register-device first")
}
return state, nil
}

func saveState(path string, state State) error {
if state.ServerURL == "" {
state.ServerURL = defaultServerURL
}
state.ServerURL = strings.TrimRight(state.ServerURL, "/")
if state.ProtocolVersion == "" {
state.ProtocolVersion = protocolVersionStub
}

dir := filepath.Dir(path)
if dir != "." && dir != "" {
if err := os.MkdirAll(dir, 0700); err != nil {
return err
}
}

body, err := json.MarshalIndent(state, "", "  ")
if err != nil {
return err
}

return os.WriteFile(path, body, 0600)
}

func serverFromStateOrFlag(statePath string, serverFlag string) (string, error) {
server := strings.TrimRight(serverFlag, "/")
if server != "" {
return server, nil
}

state, err := loadState(statePath)
if err == nil && state.ServerURL != "" {
return strings.TrimRight(state.ServerURL, "/"), nil
}

return defaultServerURL, nil
}

func postJSON(url string, req any, out any) error {
body, err := json.Marshal(req)
if err != nil {
return err
}

httpResp, err := http.Post(url, "application/json", bytes.NewReader(body))
if err != nil {
return err
}
defer httpResp.Body.Close()

return decodeResponse(httpResp, out)
}

func getJSON(url string, out any) error {
httpResp, err := http.Get(url)
if err != nil {
return err
}
defer httpResp.Body.Close()

return decodeResponse(httpResp, out)
}

func decodeResponse(resp *http.Response, out any) error {
body, err := io.ReadAll(resp.Body)
if err != nil {
return err
}

if resp.StatusCode < 200 || resp.StatusCode >= 300 {
var er ErrorResponse
if json.Unmarshal(body, &er) == nil && er.Error.Code != "" {
return fmt.Errorf("%s: %s", er.Error.Code, er.Error.Message)
}
return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
}

if out == nil {
return nil
}

if err := json.Unmarshal(body, out); err != nil {
return fmt.Errorf("decode response: %w; body=%s", err, string(body))
}

return nil
}

func sanitizeLabel(value string) string {
value = strings.TrimSpace(strings.ToLower(value))
value = strings.ReplaceAll(value, " ", "-")
if value == "" {
return "device"
}
return value
}
