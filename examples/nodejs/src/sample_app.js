import http from 'http';
import https from 'https';
import os from 'os';
import axios from 'axios'; // Axios for easy HTTP requests
import { URL } from 'url';
const { version } = process;

// Constants
const remoteUrl = "https://whoami.local.test";
const hostname = os.hostname();
const nodeVersion = version;

// Helper functions
async function fetchWithNodeFetch(url) {
    const response = await fetch(url);
    if (!response.ok) {
        throw new Error(`HTTP Error ${response.status}`);
    }
    return parseContent(await response.text());
}

async function fetchWithAxios(url) {
    const response = await axios.get(url);
    return parseContent(response.data);
}

async function fetchWithHttps(url) {
    return new Promise((resolve, reject) => {
        const parsedUrl = new URL(url);
        const options = { method: 'GET' };
        const req = https.request(parsedUrl, options, (res) => {
            if (res.statusCode >= 400) {
                reject(new Error(`HTTP Error ${res.statusCode}`));
                return;
            }
            let data = '';
            res.on('data', (chunk) => (data += chunk));
            res.on('end', () => resolve(parseContent(data)));
        });
        req.on('error', reject);
        req.end();
    });
}

// Utility functions
function parseContent(content) {
    return content
        .split('\n')
        .filter(expectedLine)
        .map(indentLine)
        .join('\n');
}

function expectedLine(line) {
    if (line.includes('Hostname')) return true;
    if (line.includes('IP') && line.includes('.') && !line.includes('127.0.0.')) return true;
    return false;
}

function indentLine(line) {
    return `    ${line}`;
}

// Main HTTP server
const server = http.createServer(async (req, res) => {
    if (req.method === 'GET' && req.url === '/') {
        let response = `Hi, I'm Node.js/${nodeVersion} service running on '${hostname}' host.\n\n`;
        response += `Time is ${new Date().toISOString()}\n\n`;
        response += `Rendering ${remoteUrl} page\n`;

        const fetchMethodsToTry = [
            ['fetch', fetchWithNodeFetch],
            ['axios', fetchWithAxios],
            ['https', fetchWithHttps],
        ];

        for (const [name, method] of fetchMethodsToTry) {
            response += `\nRequest using '${name}' lib:\n\n`;
            try {
                response += await method(remoteUrl) + '\n';
            } catch (error) {
                response += indentLine(`# Error: (${error.name}) ${error.message}\n`);
            }
        }

        response += '\nThank you!\n';

        res.writeHead(200, { 'Content-Type': 'text/plain' });
        res.end(response);
    } else {
        res.writeHead(404, { 'Content-Type': 'text/plain' });
        res.end('Not Found');
    }
});

// Run the server
const PORT = 8080;
server.listen(PORT, () => {
    console.log(`Server running on http://localhost:${PORT}/`);
});
