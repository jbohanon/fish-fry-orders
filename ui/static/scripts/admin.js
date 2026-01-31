// WebSocket connection for real-time stats updates
let wsHandler = null;

document.addEventListener('DOMContentLoaded', function() {
    // Setup collapsible sections
    setupCollapsibleSections();

    // Handle menu item actions
    document.querySelectorAll('.edit-item').forEach(button => {
        button.addEventListener('click', function() {
            const card = this.closest('.menu-item-card');
            const itemId = card.dataset.itemId;
            showEditItemModal(itemId);
        });
    });

    document.querySelectorAll('.delete-item').forEach(button => {
        button.addEventListener('click', function() {
            const card = this.closest('.menu-item-card');
            const itemId = card.dataset.itemId;
            if (confirm('Are you sure you want to delete this menu item?')) {
                deleteMenuItem(itemId);
            }
        });
    });

    // Add menu item button
    document.getElementById('add-menu-item')?.addEventListener('click', () => {
        showEditItemModal();
    });

    // Update passwords button
    document.getElementById('update-passwords')?.addEventListener('click', updatePasswords);

    // Menu item reordering
    setupMenuReordering();

    // Purge orders buttons
    document.getElementById('purge-orders-today')?.addEventListener('click', () => {
        showPurgeModal('today');
    });
    document.getElementById('purge-all-orders')?.addEventListener('click', () => {
        showPurgeModal('all');
    });

    // Initialize WebSocket connection for real-time stats updates
    initializeWebSocket();
});

// Initialize WebSocket connection using shared event handler
function initializeWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/orders`;
    
    wsHandler = new WebSocketEventHandler(wsUrl, {
        reconnectDelay: 5000,
        onOpen: () => {
            console.log('Admin dashboard WebSocket connected');
        },
        onClose: () => {
            console.log('Admin dashboard WebSocket disconnected');
        },
        onError: (error) => {
            console.error('Admin dashboard WebSocket error:', error);
        }
    });

    // Listen for stats updates
    wsHandler.on('stats_update', (data) => {
        updateStats(data.stats);
    });

    // Also listen for order updates to trigger stats recalculation
    wsHandler.on('order_new', () => {
        // Stats will be updated via stats_update event
    });

    wsHandler.on('order_update', () => {
        // Stats will be updated via stats_update event
    });
}

// Update statistics display
function updateStats(stats) {
    if (!stats) return;

    // Update Total Orders
    const totalOrdersEl = document.querySelector('.stat-card:nth-child(1) .stat-value');
    if (totalOrdersEl && stats.totalOrders !== undefined) {
        totalOrdersEl.textContent = stats.totalOrders;
    }

    // Update Orders Today
    const ordersTodayEl = document.querySelector('.stat-card:nth-child(2) .stat-value');
    if (ordersTodayEl && stats.ordersToday !== undefined) {
        ordersTodayEl.textContent = stats.ordersToday;
    }

    // Update Revenue
    const revenueEl = document.querySelector('.stat-card:nth-child(3) .stat-value');
    if (revenueEl && stats.revenue !== undefined) {
        revenueEl.textContent = `$${stats.revenue.toFixed(2)}`;
    }
}

// Show edit menu item modal
function showEditItemModal(itemId = null) {
    const modal = document.createElement('div');
    modal.className = 'modal';
    modal.innerHTML = `
        <div class="modal-content">
            <h3>${itemId ? 'Edit' : 'Add'} Menu Item</h3>
            <form id="menu-item-form">
                <div class="form-group">
                    <label for="item-name">Name</label>
                    <input type="text" id="item-name" required>
                </div>
                <div class="form-group">
                    <label for="item-price">Price</label>
                    <input type="number" id="item-price" step="0.01" required>
                </div>
                <div class="form-actions">
                    <button type="button" class="btn-secondary" onclick="this.closest('.modal').remove()">Cancel</button>
                    <button type="submit" class="btn-primary">Save</button>
                </div>
            </form>
        </div>
    `;

    document.body.appendChild(modal);

    // Load item data if editing
    if (itemId) {
        loadMenuItemData(itemId);
    }

    // Handle form submission
    modal.querySelector('form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = {
            name: document.getElementById('item-name').value,
            price: parseFloat(document.getElementById('item-price').value)
        };

        try {
            if (itemId) {
                await updateMenuItem(itemId, formData);
            } else {
                await createMenuItem(formData);
            }
            modal.remove();
            window.location.reload();
        } catch (error) {
            console.error('Failed to save menu item:', error);
            showToast('Failed to save menu item', 'error');
        }
    });
}

// Load menu item data for editing
async function loadMenuItemData(itemId) {
    try {
        const response = await fetch(`/api/menu-items/${itemId}`, {
            credentials: 'include'
        });

        if (!response.ok) {
            throw new Error('Failed to load menu item');
        }

        const item = await response.json();
        document.getElementById('item-name').value = item.name;
        document.getElementById('item-price').value = item.price;
    } catch (error) {
        console.error('Failed to load menu item:', error);
        showToast('Failed to load menu item', 'error');
    }
}

// Create new menu item
async function createMenuItem(data) {
    const response = await fetch('/api/menu-items', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data),
        credentials: 'include'
    });

    if (!response.ok) {
        throw new Error('Failed to create menu item');
    }
}

// Update menu item
async function updateMenuItem(itemId, data) {
    const response = await fetch(`/api/menu-items/${itemId}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data),
        credentials: 'include'
    });

    if (!response.ok) {
        throw new Error('Failed to update menu item');
    }
}

// Delete menu item
async function deleteMenuItem(itemId) {
    try {
        const response = await fetch(`/api/menu-items/${itemId}`, {
            method: 'DELETE',
            credentials: 'include'
        });

        if (!response.ok) {
            throw new Error('Failed to delete menu item');
        }

        // Remove item from UI
        document.querySelector(`[data-item-id="${itemId}"]`).remove();
        showToast('Menu item deleted successfully');
    } catch (error) {
        console.error('Failed to delete menu item:', error);
        showToast('Failed to delete menu item', 'error');
    }
}

// Update passwords
async function updatePasswords() {
    const workerPassword = document.getElementById('worker-password').value;
    const adminPassword = document.getElementById('admin-password').value;

    if (!workerPassword && !adminPassword) {
        showToast('Please enter at least one password to update', 'error');
        return;
    }

    try {
        const response = await fetch('/api/admin/passwords', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                workerPassword,
                adminPassword
            }),
            credentials: 'include'
        });

        if (!response.ok) {
            throw new Error('Failed to update passwords');
        }

        // Clear password fields
        document.getElementById('worker-password').value = '';
        document.getElementById('admin-password').value = '';
        showToast('Passwords updated successfully');
    } catch (error) {
        console.error('Failed to update passwords:', error);
        showToast('Failed to update passwords', 'error');
    }
}

// Setup collapsible sections
function setupCollapsibleSections() {
    document.querySelectorAll('.admin-section-header').forEach(header => {
        header.addEventListener('click', function() {
            const section = this.closest('.admin-section');
            section.classList.toggle('collapsed');
        });
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

// Setup menu item reordering
function setupMenuReordering() {
    const menuList = document.getElementById('menu-items-list');
    if (!menuList) return;

    // Handle move up/down buttons
    menuList.addEventListener('click', function(e) {
        if (e.target.classList.contains('move-up')) {
            moveMenuItem(e.target.closest('.menu-item-card'), -1);
        } else if (e.target.classList.contains('move-down')) {
            moveMenuItem(e.target.closest('.menu-item-card'), 1);
        }
    });

    // Simple drag and drop (optional enhancement)
    let draggedElement = null;
    menuList.addEventListener('mousedown', function(e) {
        if (e.target.classList.contains('menu-item-drag-handle')) {
            draggedElement = e.target.closest('.menu-item-card');
            draggedElement.style.opacity = '0.5';
        }
    });

    menuList.addEventListener('mouseup', function() {
        if (draggedElement) {
            draggedElement.style.opacity = '1';
            draggedElement = null;
        }
    });

    menuList.addEventListener('mouseover', function(e) {
        if (draggedElement && e.target.closest('.menu-item-card') && e.target.closest('.menu-item-card') !== draggedElement) {
            const target = e.target.closest('.menu-item-card');
            const allCards = Array.from(menuList.querySelectorAll('.menu-item-card'));
            const draggedIndex = allCards.indexOf(draggedElement);
            const targetIndex = allCards.indexOf(target);
            
            if (draggedIndex < targetIndex) {
                menuList.insertBefore(draggedElement, target.nextSibling);
            } else {
                menuList.insertBefore(draggedElement, target);
            }
            saveMenuOrder();
        }
    });
}

// Move menu item up or down
function moveMenuItem(card, direction) {
    const menuList = document.getElementById('menu-items-list');
    const allCards = Array.from(menuList.querySelectorAll('.menu-item-card'));
    const currentIndex = allCards.indexOf(card);
    const newIndex = currentIndex + direction;

    if (newIndex < 0 || newIndex >= allCards.length) {
        return; // Can't move further
    }

    if (direction < 0) {
        menuList.insertBefore(card, allCards[newIndex]);
    } else {
        menuList.insertBefore(card, allCards[newIndex].nextSibling);
    }

    saveMenuOrder();
}

// Save menu item order
async function saveMenuOrder() {
    const menuList = document.getElementById('menu-items-list');
    const cards = Array.from(menuList.querySelectorAll('.menu-item-card'));
    
    const itemOrders = {};
    cards.forEach((card, index) => {
        itemOrders[card.dataset.itemId] = index + 1;
    });

    try {
        const response = await fetch('/api/menu-items/order', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ itemOrders }),
            credentials: 'include'
        });

        if (!response.ok) {
            throw new Error('Failed to update menu order');
        }

        showToast('Menu order updated');
    } catch (error) {
        console.error('Failed to save menu order:', error);
        showToast('Failed to update menu order', 'error');
        // Reload to restore correct order
        window.location.reload();
    }
}

// Show purge orders confirmation modal
function showPurgeModal(scope) {
    const scopeText = scope === 'today' ? "today's" : 'all';
    const modal = document.createElement('div');
    modal.className = 'modal';
    modal.innerHTML = `
        <div class="modal-content">
            <h3>Confirm Purge Orders</h3>
            <p>Are you sure you want to delete <strong>${scopeText}</strong> orders?</p>
            <p class="warning-text">This action cannot be undone!</p>
            <div class="form-actions">
                <button type="button" class="btn-secondary" onclick="this.closest('.modal').remove()">Cancel</button>
                <button type="button" class="btn-danger" id="confirm-purge">Yes, Delete ${scope === 'today' ? "Today's" : 'All'} Orders</button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);

    // Handle confirmation
    modal.querySelector('#confirm-purge').addEventListener('click', async () => {
        try {
            await purgeOrders(scope);
            modal.remove();
        } catch (error) {
            console.error('Failed to purge orders:', error);
            showToast('Failed to purge orders', 'error');
        }
    });
}

// Purge orders
async function purgeOrders(scope) {
    try {
        const response = await fetch('/api/orders/purge', {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ scope }),
            credentials: 'include'
        });

        if (!response.ok) {
            throw new Error('Failed to purge orders');
        }

        const data = await response.json();
        showToast(`Successfully deleted ${data.deleted} order(s)`);
        
        // Reload page to reflect changes
        setTimeout(() => {
            window.location.reload();
        }, 1500);
    } catch (error) {
        console.error('Failed to purge orders:', error);
        throw error;
    }
} 