/**
 * Login Page JavaScript
 * Handles user authentication
 */

document.addEventListener('DOMContentLoaded', function() {
    const loginForm = document.getElementById('loginForm');
    if (loginForm) {
        loginForm.addEventListener('submit', handleLoginSubmit);
    }

    // Focus on username input
    const usernameInput = document.getElementById('username');
    if (usernameInput) {
        usernameInput.focus();
    }
});

/**
 * Handle login form submission
 * @param {Event} e - Submit event
 */
async function handleLoginSubmit(e) {
    e.preventDefault();
    
    const error = document.getElementById('error');
    if (error) {
        error.classList.remove('show');
    }
    
    const usernameInput = document.getElementById('username');
    const passwordInput = document.getElementById('password');
    
    const username = usernameInput?.value?.trim();
    const password = passwordInput?.value;
    
    if (!username || !password) {
        showError('Please enter both username and password');
        return;
    }
    
    try {
        showLoadingSpinner(true, 'Logging in...');
        
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password }),
            credentials: 'include'
        });
        
        showLoadingSpinner(false);
        
        if (response.ok) {
            // Redirect to dashboard
            window.location.href = '/dashboard-new';
        } else {
            const result = await response.json();
            const errorMessage = result.error || 'Invalid credentials';
            showError(errorMessage);
            
            // Clear password field on error
            if (passwordInput) {
                passwordInput.value = '';
                passwordInput.focus();
            }
        }
    } catch (err) {
        showLoadingSpinner(false);
        showError('Connection error. Please try again.');
        console.error('Login error:', err);
    }
}

/**
 * Show error message
 * @param {string} message - Error message
 */
function showError(message) {
    const errorEl = document.getElementById('error');
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.classList.add('show');
        
        // Auto-hide after 5 seconds
        setTimeout(() => {
            errorEl.classList.remove('show');
        }, 5000);
    } else {
        // Fallback to alert if error element not found
        alert(message);
    }
}

/**
 * Clear login form
 */
function clearLoginForm() {
    const loginForm = document.getElementById('loginForm');
    if (loginForm) {
        loginForm.reset();
    }
    const usernameInput = document.getElementById('username');
    if (usernameInput) {
        usernameInput.focus();
    }
}
