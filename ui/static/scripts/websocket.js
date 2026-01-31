/**
 * Shared WebSocket event handler utility
 * Provides a reusable pattern for WebSocket connections with event-based message handling
 */

class WebSocketEventHandler {
    constructor(url, options = {}) {
        this.url = url;
        this.options = {
            reconnectDelay: options.reconnectDelay || 5000,
            maxReconnectAttempts: options.maxReconnectAttempts || Infinity,
            autoReconnect: options.autoReconnect !== false,
            ...options
        };
        
        this.ws = null;
        this.eventHandlers = new Map();
        this.reconnectAttempts = 0;
        this.isManualClose = false;
        
        this.connect();
    }

    connect() {
        try {
            this.ws = new WebSocket(this.url);
            this.setupEventHandlers();
        } catch (error) {
            console.error('WebSocket connection error:', error);
            this.scheduleReconnect();
        }
    }

    setupEventHandlers() {
        this.ws.onopen = () => {
            console.log('WebSocket connection established');
            this.reconnectAttempts = 0;
            this.onOpen();
        };

        this.ws.onclose = () => {
            console.log('WebSocket connection closed');
            this.onClose();
            
            if (!this.isManualClose && this.options.autoReconnect) {
                this.scheduleReconnect();
            }
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.onError(error);
        };

        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.handleMessage(data);
            } catch (error) {
                console.error('Error parsing WebSocket message:', error);
            }
        };
    }

    handleMessage(data) {
        // Handle message by type
        if (data.type) {
            const handlers = this.eventHandlers.get(data.type) || [];
            handlers.forEach(handler => {
                try {
                    handler(data);
                } catch (error) {
                    console.error(`Error in handler for ${data.type}:`, error);
                }
            });
        }
        
        // Also call generic message handler
        this.onMessage(data);
    }

    // Public API: Register event handlers
    on(eventType, handler) {
        if (!this.eventHandlers.has(eventType)) {
            this.eventHandlers.set(eventType, []);
        }
        this.eventHandlers.get(eventType).push(handler);
        return this; // Allow chaining
    }

    // Public API: Remove event handler
    off(eventType, handler) {
        if (this.eventHandlers.has(eventType)) {
            const handlers = this.eventHandlers.get(eventType);
            const index = handlers.indexOf(handler);
            if (index > -1) {
                handlers.splice(index, 1);
            }
        }
        return this;
    }

    // Public API: Send message
    send(data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(data));
        } else {
            console.warn('WebSocket is not open. Message not sent:', data);
        }
    }

    // Public API: Close connection
    close() {
        this.isManualClose = true;
        if (this.ws) {
            this.ws.close();
        }
    }

    // Override these in subclasses or via options
    onOpen() {
        if (this.options.onOpen) {
            this.options.onOpen();
        }
    }

    onClose() {
        if (this.options.onClose) {
            this.options.onClose();
        }
    }

    onError(error) {
        if (this.options.onError) {
            this.options.onError(error);
        }
    }

    onMessage(data) {
        if (this.options.onMessage) {
            this.options.onMessage(data);
        }
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.options.maxReconnectAttempts) {
            console.error('Max reconnect attempts reached');
            if (this.options.onMaxReconnectAttempts) {
                this.options.onMaxReconnectAttempts();
            }
            return;
        }

        this.reconnectAttempts++;
        console.log(`Scheduling reconnect attempt ${this.reconnectAttempts} in ${this.options.reconnectDelay}ms`);
        
        setTimeout(() => {
            if (!this.isManualClose) {
                this.connect();
            }
        }, this.options.reconnectDelay);
    }

    // Get connection state
    get readyState() {
        return this.ws ? this.ws.readyState : WebSocket.CLOSED;
    }

    // Check if connected
    isConnected() {
        return this.ws && this.ws.readyState === WebSocket.OPEN;
    }
}

// Export for use in other scripts
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WebSocketEventHandler;
}
