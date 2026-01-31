document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('new-order-form');
    const orderItems = document.getElementById('order-items');
    const addItemBtn = document.getElementById('add-item');
    const orderIdDisplay = document.getElementById('order-id-display');
    const orderIdNumber = document.getElementById('order-id-number');
    const createAnotherBtn = document.getElementById('create-another');

    // Get the menu options from the first select (template) and store them
    const firstSelect = document.querySelector('.menu-item');
    const menuOptionsHTML = Array.from(firstSelect.options)
        .map(opt => `<option value="${opt.value}">${opt.text}</option>`)
        .join('');

    // Function to reset form for creating another order
    function resetForm() {
        form.reset();
        form.style.display = 'block';
        orderIdDisplay.style.display = 'none';
        
        // Reset order items to single empty item
        orderItems.innerHTML = `
            <div class="order-item">
                <select class="menu-item" required>
                    ${menuOptionsHTML}
                </select>
                <input type="number" class="quantity" min="1" value="1" required>
                <button type="button" class="remove-item">Remove</button>
            </div>
        `;
        
        // Focus on vehicle description field
        document.getElementById('vehicle-description').focus();
    }

    // Handle "Create Another Order" button
    createAnotherBtn.addEventListener('click', resetForm);

    // Function to show a temporary message
    function showMessage(message, type = 'info') {
        // Remove existing message if any
        const existingMsg = document.getElementById('order-message');
        if (existingMsg) {
            existingMsg.remove();
        }

        const msgDiv = document.createElement('div');
        msgDiv.id = 'order-message';
        msgDiv.className = `order-message order-message-${type}`;
        msgDiv.textContent = message;
        
        // Insert at the top of the form
        form.insertBefore(msgDiv, form.firstChild);
        
        // Auto-remove after 3 seconds
        setTimeout(() => {
            if (msgDiv.parentNode) {
                msgDiv.style.opacity = '0';
                msgDiv.style.transform = 'translateY(-10px)';
                setTimeout(() => msgDiv.remove(), 300);
            }
        }, 3000);
    }

    // Function to check if an item is already selected
    function findExistingItem(menuItemId) {
        const items = document.querySelectorAll('.order-item');
        for (let item of items) {
            const select = item.querySelector('.menu-item');
            if (select && select.value === menuItemId && menuItemId !== '') {
                return item;
            }
        }
        return null;
    }

    // Function to highlight an item row briefly
    function highlightItem(itemRow) {
        itemRow.classList.add('item-highlight');
        itemRow.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        setTimeout(() => {
            itemRow.classList.remove('item-highlight');
        }, 2000);
    }

    // Add new item row
    addItemBtn.addEventListener('click', function() {
        const newItem = document.createElement('div');
        newItem.className = 'order-item';
        newItem.innerHTML = `
            <select class="menu-item" required>
                ${menuOptionsHTML}
            </select>
            <input type="number" class="quantity" min="1" value="1" required>
            <button type="button" class="remove-item">Remove</button>
        `;
        orderItems.appendChild(newItem);
        
        // Focus on the new select
        newItem.querySelector('.menu-item').focus();
    });

    // Handle item selection - check for duplicates
    orderItems.addEventListener('change', function(e) {
        if (e.target.classList.contains('menu-item')) {
            const selectedValue = e.target.value;
            if (!selectedValue) return; // Empty selection is fine
            
            // Find all other items with the same selection
            const allItems = document.querySelectorAll('.order-item');
            let duplicateFound = false;
            let existingItem = null;
            
            for (let item of allItems) {
                if (item === e.target.closest('.order-item')) continue; // Skip current item
                
                const otherSelect = item.querySelector('.menu-item');
                if (otherSelect && otherSelect.value === selectedValue) {
                    duplicateFound = true;
                    existingItem = item;
                    break;
                }
            }
            
            if (duplicateFound && existingItem) {
                // Get the item name for the message
                const selectedOption = e.target.options[e.target.selectedIndex];
                const itemName = selectedOption.text.split(' - ')[0]; // Get name without price
                
                // Increase quantity of existing item
                const existingQuantity = existingItem.querySelector('.quantity');
                const currentQty = parseInt(existingQuantity.value) || 1;
                existingQuantity.value = currentQty + 1;
                
                // Highlight the existing item
                highlightItem(existingItem);
                
                // Show message
                showMessage(`${itemName} already in order. Quantity increased to ${currentQty + 1}.`, 'info');
                
                // Reset the current select to empty
                e.target.value = '';
                
                // Focus back on the select that was just reset
                setTimeout(() => e.target.focus(), 100);
            }
        }
    });

    // Remove item row
    orderItems.addEventListener('click', function(e) {
        if (e.target.classList.contains('remove-item')) {
            // Ensure at least one item remains
            if (orderItems.children.length > 1) {
                e.target.closest('.order-item').remove();
            } else {
                showMessage('An order must have at least one item.', 'error');
            }
        }
    });

    // Handle form submission
    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        const vehicleDescription = document.getElementById('vehicle-description').value;
        const items = Array.from(document.querySelectorAll('.order-item')).map(item => ({
            menuItemId: item.querySelector('.menu-item').value,
            quantity: parseInt(item.querySelector('.quantity').value)
        }));

        if (items.some(item => !item.menuItemId)) {
            alert('Please select an item for all order items');
            return;
        }

        try {
            const response = await fetch('/api/orders', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    customerName: vehicleDescription,
                    items
                })
            });

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.message || 'Failed to create order');
            }

            const orderData = await response.json();
            
            // Hide the form and show the order ID prominently
            form.style.display = 'none';
            orderIdNumber.textContent = `#${orderData.id}`;
            orderIdDisplay.style.display = 'block';
            
            // Scroll to top to show the order ID
            window.scrollTo({ top: 0, behavior: 'smooth' });
        } catch (error) {
            console.error('Error creating order:', error);
            alert('Failed to create order. Please try again.');
        }
    });
}); 