<?php
/**
 * CSRF Token Endpoint
 * Returns a fresh CSRF token for the current session
 */

require_once __DIR__ . '/../includes/auth.php';
requireAuth();

header('Content-Type: application/json');

$token = CSRFProtection::generateToken();

echo json_encode([
    'success' => true,
    'token' => $token
]);
