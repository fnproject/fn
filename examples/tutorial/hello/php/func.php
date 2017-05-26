<?php
require 'vendor/autoload.php';

fwrite(STDERR, "--> this will go to stderr (server logs)\n");
stream_set_blocking(STDIN, 0);
$payload = json_decode(file_get_contents("php://stdin"), true);
if (isset($payload['name'])) {
    echo "Hello ", $payload['name'],"!\n";
} else {
    echo "Hello World!\n";
}
