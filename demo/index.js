'use strict'

const http = require('http')
const os = require('os')
const allIfcs = os.networkInterfaces()

const ifcs = {}
const port = process.env.PORT || 3000
const server = http.createServer((req, res) => {
  res.writeHead(200, {
    'Content-Type': 'application/json'
  })
  res.end(JSON.stringify({
    env: process.env,
    ips: ifcs,
    req: {
      headers: req.headers,
      method: req.method,
      url: req.url
    }
  }, null, 2))
})

// Generate the list of IPs
Object.keys(allIfcs).forEach((name) => {
  allIfcs[name].forEach((ifc) => {
    if (ifc.family === 'IPv4')
      ifcs[name] = ifc.address
  })
})

server.listen(port, () => {
  console.log('Current Environment')
  console.log('-------------------')

  Object.keys(process.env).forEach((key) => {
    console.log('  %s: %s', key, process.env[key])
  })

  console.log();
  console.log('Current IPs')
  console.log('-----------');

  Object.keys(ifcs).forEach((name) => {
    console.log('  %s: %s', name, ifcs[name]);
  })

  console.log('Server listening on port', port)
})
