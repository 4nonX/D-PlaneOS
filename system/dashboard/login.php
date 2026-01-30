<?php
require_once __DIR__ . '/includes/auth.php';

// If already logged in, redirect to dashboard
if (isAuthenticated()) {
    header('Location: /');
    exit;
}

$error = '';

if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $clientIP = $_SERVER['REMOTE_ADDR'] ?? 'unknown';
    $username = validateInput($_POST['username'] ?? '', 'username');
    $password = $_POST['password'] ?? '';
    
    if (!$username || !$password) {
        $error = 'Please enter both username and password';
    } else {
        $db = getDB();
        $stmt = $db->prepare('SELECT id, username, password, role FROM users WHERE username = ?');
        $stmt->execute([$username]);
        $user = $stmt->fetch();
        
        if ($user && password_verify($password, $user['password'])) {
            // Regenerate session ID to prevent fixation attacks
            session_regenerate_id(true);
            
            // Login successful - record success
            BruteForceProtection::recordAttempt($clientIP, $username, true);
            
            $_SESSION['user_id'] = $user['id'];
            $_SESSION['username'] = $user['username'];
            $_SESSION['user_role'] = $user['role'] ?? 'user';  // Store role in session
            $_SESSION['last_activity'] = time();
            
            // Update last login
            $stmt = $db->prepare('UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?');
            $stmt->execute([$user['id']]);
            
            logAction('login');
            
            header('Location: /');
            exit;
        } else {
            // Login failed - record failure (may trigger ban)
            BruteForceProtection::recordAttempt($clientIP, $username, false);
            
            $error = 'Invalid username or password';
            logAction('login_failed', 'user', $username, 'Invalid credentials from ' . $clientIP);
        }
    }
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>D-PlaneOS - Login</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .login-box {
            background: rgba(255, 255, 255, 0.95);
            backdrop-filter: blur(10px);
            border-radius: 16px;
            padding: 40px;
            width: 100%;
            max-width: 400px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
        }
        h1 {
            text-align: center;
            font-size: 32px;
            margin-bottom: 10px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .subtitle {
            text-align: center;
            color: #666;
            font-size: 14px;
            margin-bottom: 30px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            margin-bottom: 8px;
            color: #333;
            font-weight: 500;
        }
        input[type="text"], input[type="password"] {
            width: 100%;
            padding: 12px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 14px;
            transition: border-color 0.3s;
        }
        input:focus {
            outline: none;
            border-color: #667eea;
        }
        button {
            width: 100%;
            padding: 14px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s;
        }
        button:hover { transform: translateY(-2px); }
        button:active { transform: translateY(0); }
        .error {
            background: #fee;
            color: #c33;
            padding: 12px;
            border-radius: 8px;
            margin-bottom: 20px;
            font-size: 14px;
            border: 1px solid #fcc;
        }
        .version {
            text-align: center;
            margin-top: 20px;
            color: #999;
            font-size: 12px;
        }
        .beta { display: inline-block; background: #ff9800; color: white; padding: 2px 6px; border-radius: 4px; font-size: 10px; margin-left: 5px; }
    </style>
</head>
<body>
    <div class="login-box">
        <h1>D-PlaneOS<span class="beta">BETA</span></h1>
        <p class="subtitle">Transparency over Abstraction</p>
        <?php if ($error): ?>
        <div class="error"><?= htmlspecialchars($error) ?></div>
        <?php endif; ?>
        <form method="POST">
            <div class="form-group">
                <label for="username">Username</label>
                <input type="text" id="username" name="username" required autofocus>
            </div>
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            <button type="submit">Login</button>
        </form>
        <div class="version">v1.5.0</div>
    </div>
</body>
</html>
