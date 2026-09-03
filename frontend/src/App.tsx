import React, { useState, useEffect } from 'react';

export default function App() {
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    // Initiate the single persistent WebSocket pipeline connection to Go Gateway
    const socket = new WebSocket('ws://localhost:8080/ws');

    socket.onopen = () => {
      addLog('[Frontend] Connected to Go Gateway WebSocket successfully!');
    };

    socket.onmessage = (event) => {
      addLog(`[Frontend] Received response from Server: ${event.data}`);
    };

    socket.onclose = () => {
      addLog('[Frontend] WebSocket pipe closed.');
    };

    setWs(socket);
    return () => socket.close();
  }, []);

  const addLog = (message: string) => {
    setLogs((prev) => [...prev, `${new Date().toLocaleTimeString()} - ${message}`]);
  };

  const triggerAction = () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      addLog('[Frontend] Error: WebSocket is not open.');
      return;
    }

    // Build the clean synchronized multi-plexed packet envelope defined in requirements
    const packet = {
      action: "game:ping",
      msg_id: crypto.randomUUID(), // Generates unique token ID for matching responses
      payload: {
        client_timestamp: Date.now()
      }
    };

    addLog(`[Frontend] Sent action packet: '${packet.action}' with msg_id: '${packet.msg_id}'`);
    ws.send(JSON.stringify(packet));
  };

  return (
    <div style={{ padding: '20px', fontFamily: 'sans-serif', background: '#121212', color: '#fff', minHeight: '100vh' }}>
      <h1>OGame Next-Gen Phase 1 (MVI)</h1>
      <button 
        onClick={triggerAction} 
        style={{ padding: '10px 20px', fontSize: '16px', cursor: 'pointer', background: '#00bcd4', border: 'none', color: '#fff', borderRadius: '4px' }}
      >
        Trigger Action
      </button>

      <h2>Network Transmission Logs:</h2>
      <div style={{ background: '#1e1e1e', padding: '15px', borderRadius: '4px', height: '400px', overflowY: 'auto' }}>
        {logs.map((log, index) => (
          <p key={index} style={{ margin: '5px 0', fontFamily: 'monospace', color: log.includes('Received') ? '#a3be8c' : '#88c0d0' }}>
            {log}
          </p>
        ))}
      </div>
    </div>
  );
}
