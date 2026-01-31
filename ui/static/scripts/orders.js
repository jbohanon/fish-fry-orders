document.addEventListener('DOMContentLoaded', function() {
    // Format timestamps to local time
    formatTimestamps();
    
    // Don't reload orders if server already rendered them - just set up WebSocket
    // The server-side rendering already has the correct sort order
    // loadOrders(); // Commented out to preserve server-side sort order
    
    // Initialize WebSocket connection for real-time updates
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/orders`);
    
    // Handle WebSocket connection
    ws.onopen = function() {
        console.log('WebSocket connection established');
    };

    ws.onclose = function() {
        console.log('WebSocket connection closed');
        // Attempt to reconnect after 5 seconds
        setTimeout(() => {
            window.location.reload();
        }, 5000);
    };

    ws.onerror = function(error) {
        console.error('WebSocket error:', error);
    };

    // Handle incoming WebSocket messages
    ws.onmessage = function(event) {
        const data = JSON.parse(event.data);
        if (data.type === 'order_update') {
            updateOrderCard(data.order);
        } else if (data.type === 'order_new') {
            addOrderCard(data.order);
        }
    };

    // Handle status advancement
    document.querySelectorAll('.advance-status').forEach(button => {
        button.addEventListener('click', async function() {
            const orderId = this.dataset.orderId;
            const currentStatus = this.dataset.currentStatus;
            
            // Determine next status
            let nextStatus;
            if (currentStatus === 'new') {
                nextStatus = 'in-progress';
            } else if (currentStatus === 'in-progress') {
                nextStatus = 'completed';
            } else {
                return; // Already completed
            }

            try {
                const response = await fetch(`/api/orders/${orderId}/status`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ status: nextStatus }),
                    credentials: 'include'
                });

                if (!response.ok) {
                    throw new Error('Failed to update order status');
                }

                // Reload page to show updated status
                window.location.reload();
            } catch (error) {
                console.error('Error updating order status:', error);
                showToast('Failed to update order status', 'error');
            }
        });
    });
});

// Update existing order row
function updateOrderCard(order) {
    const row = document.querySelector(`[data-order-id="${order.id}"]`);
    if (row) {
        // Update status
        row.classList.remove('status-new', 'status-in-progress', 'status-completed');
        row.classList.add(`status-${order.status}`);
        
        // Update status badge
        const statusBadge = row.querySelector('.status-badge');
        if (statusBadge) {
            statusBadge.className = `status-badge status-${order.status}`;
            statusBadge.textContent = order.status;
        }
        
        // Update button
        const button = row.querySelector('.advance-status');
        if (button) {
            button.dataset.currentStatus = order.status;
            if (order.status === 'new') {
                button.textContent = 'Start Order';
            } else if (order.status === 'in-progress') {
                button.textContent = 'Complete Order';
            } else {
                button.textContent = '—';
                button.disabled = true;
            }
        }
        
        // Note: We don't update createdAt here because it never changes
        // and the server-rendered value is already correctly formatted
    }
}

// Create order row element (doesn't add to DOM)
function createOrderRow(order) {
    const row = document.createElement('tr');
    row.className = `order-row status-${order.status}`;
    row.dataset.orderId = order.id;
    
    const itemsList = order.items.map(item => 
        `<li>${item.quantity}x ${item.menuItemName || item.menu_item_name || item.menu_item_id || item.menuItemId || 'Unknown'}</li>`
    ).join('');
    
    let buttonText = '—';
    let buttonDisabled = '';
    if (order.status === 'new') {
        buttonText = 'Start Order';
    } else if (order.status === 'in-progress') {
        buttonText = 'Complete Order';
    } else {
        buttonDisabled = 'disabled';
    }
    
    // Format created date - handle both camelCase and snake_case
    const createdAtRaw = order.createdAt || order.created_at || '';
    let createdAtFormatted = '—';
    if (createdAtRaw) {
        const date = new Date(createdAtRaw);
        createdAtFormatted = date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    }
    
    row.innerHTML = `
        <td><a href="/orders/${order.id}" class="order-link">#${order.id}</a></td>
        <td>${order.customerName || order.vehicleDescription}</td>
        <td><ul class="items-list">${itemsList}</ul></td>
        <td><span class="status-badge status-${order.status}">${order.status}</span></td>
        <td class="created-at">${createdAtFormatted}</td>
        <td>
            <button class="btn-small advance-status" data-order-id="${order.id}" data-current-status="${order.status}" ${buttonDisabled}>
                ${buttonText}
            </button>
        </td>
    `;

    // Add event listener for status advancement
    const button = row.querySelector('.advance-status');
    if (button && !buttonDisabled) {
        button.addEventListener('click', async function() {
            const orderId = this.dataset.orderId;
            const currentStatus = this.dataset.currentStatus;
            
            let nextStatus;
            if (currentStatus === 'new') {
                nextStatus = 'in-progress';
            } else if (currentStatus === 'in-progress') {
                nextStatus = 'completed';
            } else {
                return;
            }

            try {
                const response = await fetch(`/api/orders/${orderId}/status`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ status: nextStatus }),
                    credentials: 'include'
                });

                if (!response.ok) {
                    throw new Error('Failed to update order status');
                }

                // Reload to get correct sort order after status change
                window.location.reload();
            } catch (error) {
                console.error('Error updating order status:', error);
                showToast('Failed to update order status', 'error');
            }
        });
    }

    return row;
}

// Add new order row (for WebSocket updates)
function addOrderCard(order) {
    const tbody = document.querySelector('.orders-table tbody');
    if (!tbody) return;

    const row = createOrderRow(order);
    
    // Find the correct position to insert based on sort order
    const statusPriority = {
        'in-progress': 1,
        'new': 2,
        'completed': 3
    };
    const orderPriority = statusPriority[order.status] || 4;
    
    // Find where to insert this order
    const existingRows = Array.from(tbody.querySelectorAll('.order-row'));
    let insertBefore = null;
    
    for (const existingRow of existingRows) {
        // Get status from class or data attribute
        let existingStatus = 'completed';
        if (existingRow.classList.contains('status-in-progress')) {
            existingStatus = 'in-progress';
        } else if (existingRow.classList.contains('status-new')) {
            existingStatus = 'new';
        }
        
        const existingPriority = statusPriority[existingStatus] || 4;
        const existingId = parseInt(existingRow.dataset.orderId);
        
        // If this order should come before the existing row
        if (orderPriority < existingPriority || 
            (orderPriority === existingPriority && order.id < existingId)) {
            insertBefore = existingRow;
            break;
        }
    }
    
    if (insertBefore) {
        tbody.insertBefore(row, insertBefore);
    } else {
        tbody.appendChild(row);
    }
}

// Load orders from API
async function loadOrders() {
    try {
        const response = await fetch('/api/orders', {
            credentials: 'include'
        });
        
        if (!response.ok) {
            throw new Error('Failed to load orders');
        }
        
        const orders = await response.json();
        const tbody = document.querySelector('.orders-table tbody');
        if (!tbody) return;
        
        // Clear existing orders (except server-rendered ones, we'll replace all)
        // Keep the "no orders" row if it exists
        const noOrdersRow = tbody.querySelector('.no-orders');
        if (noOrdersRow) {
            tbody.innerHTML = '';
        } else {
            // Remove all order rows but keep structure
            const rows = tbody.querySelectorAll('.order-row');
            rows.forEach(row => row.remove());
        }
        
        // Sort orders: in-progress first, then new, then completed, then by ID
        const statusPriority = {
            'in-progress': 1,
            'new': 2,
            'completed': 3
        };
        
        orders.sort((a, b) => {
            const aPriority = statusPriority[a.status] || 4;
            const bPriority = statusPriority[b.status] || 4;
            if (aPriority !== bPriority) {
                return aPriority - bPriority;
            }
            // Within same status, sort by ID ascending
            return a.id - b.id;
        });
        
        // Add each order in sorted order (append to maintain order)
        orders.forEach(order => {
            const row = createOrderRow(order);
            tbody.appendChild(row);
        });
    } catch (error) {
        console.error('Error loading orders:', error);
    }
}

// Format timestamps to local time
function formatTimestamps() {
    document.querySelectorAll('.created-at[data-timestamp]').forEach(el => {
        const timestamp = el.dataset.timestamp;
        if (timestamp) {
            const date = new Date(timestamp);
            el.textContent = date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
        }
    });
}

// Show toast notification
function showToast(message, type = 'success') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);

    setTimeout(() => {
        toast.remove();
    }, 3000);
} 