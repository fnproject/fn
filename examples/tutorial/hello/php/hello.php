<?php
require 'vendor/autoload.php';

stream_set_blocking(STDIN, 0);
$payload = json_decode(file_get_contents("php://stdin"), true);
if (isset($payload['name'])) {
    echo "Hello ", $payload['name'],"!\n\n";
} else {
    echo "Hello World!\n\n";
}
