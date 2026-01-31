document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('loginForm');
    const logoutButton = document.getElementById('logout');

    // Check initial auth state
    checkAuthState();

    // Handle form submission
    if (form) {
        form.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const password = document.getElementById('password').value;
            
            try {
                const response = await fetch('/api/auth/login', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ password }),
                    credentials: 'include'
                });

                if (!response.ok) {
                    throw new Error('Login failed');
                }

                const data = await response.json();
                
                // Redirect based on role
                if (data.role === 'admin') {
                    window.location.href = '/admin';
                } else {
                    window.location.href = '/orders';
                }
            } catch (error) {
                console.error('Login error:', error);
                showToast('Invalid password. Please try again.', 'error');
            }
        });
    }

    // Handle logout
    if (logoutButton) {
        logoutButton.addEventListener('click', async function() {
            try {
                const response = await fetch('/api/auth/logout', {
                    method: 'POST',
                    credentials: 'include'
                });

                if (!response.ok) {
                    throw new Error('Logout failed');
                }

                window.location.href = '/auth';
            } catch (error) {
                console.error('Logout error:', error);
                showToast('Failed to logout', 'error');
            }
        });
    }

    // Check auth state
    async function checkAuthState() {
        try {
            const response = await fetch('/api/auth/check', {
                method: 'GET',
                credentials: 'include'
            });

            if (response.ok) {
                const data = await response.json();
                updateAuthStatus(true, data.role);
            } else {
                updateAuthStatus(false);
            }
        } catch (error) {
            console.error('Failed to check auth state:', error);
            updateAuthStatus(false);
        }
    }

    // Update auth status display
    function updateAuthStatus(isAuthenticated, role = null) {
        const authStatus = document.querySelector('.auth-status');
        if (authStatus) {
            if (isAuthenticated) {
                authStatus.innerHTML = `
                    <p>Logged in as: <strong>${role}</strong></p>
                    <button id="logout" class="btn-secondary">Logout</button>
                `;
                document.getElementById('logout').addEventListener('click', async () => {
                    try {
                        const response = await fetch('/api/auth/logout', {
                            method: 'POST',
                            credentials: 'include'
                        });

                        if (!response.ok) {
                            throw new Error('Logout failed');
                        }

                        window.location.href = '/auth';
                    } catch (error) {
                        console.error('Logout error:', error);
                        showToast('Failed to logout', 'error');
                    }
                });
            } else {
                authStatus.innerHTML = '';
            }
        }
    }
});

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