document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('status-update-form');
    const orderId = window.location.pathname.split('/').pop();
    
    form.addEventListener('submit', async function(e) {
        e.preventDefault();
        
        const status = document.getElementById('status-select').value;
        
        try {
            const response = await fetch(`/api/orders/${orderId}/status`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ status })
            });

            if (!response.ok) {
                throw new Error('Failed to update order status');
            }

            showToast('Order status updated successfully');
            // Reload page to show updated status
            setTimeout(() => {
                window.location.reload();
            }, 1000);
        } catch (error) {
            console.error('Error updating order status:', error);
            showToast('Failed to update order status', 'error');
        }
    });
});

function showToast(message, type = 'success') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);

    setTimeout(() => {
        toast.remove();
    }, 3000);
}
