/*
Copyright © 2016 Apigee Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

'use strict'

const http = require('http')
const os = require('os')
const socketIO = require('socket.io')
const allIfcs = os.networkInterfaces()

const basePath = process.env.BASE_PATH || '/nodejs'
const getEnv = (req) => {
  var env = {
    env: process.env,
    ips: ifcs
  }

  if (typeof req !== 'undefined') {
    env.req = {
      headers: req.headers,
      method: req.method,
      url: req.url
    }
  }

  return env
}
const ifcs = {}
const port = process.env.PORT || 3000
const server = http.createServer((req, res) => {
  console.log(req)
  res.writeHead(200, {
    'Content-Type': 'application/json'
  })
  res.end(JSON.stringify(getEnv(req), null, 2))
})
const io = socketIO(server, {path: basePath + '/socket.io'})

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

io.on('connection', function (socket) {
  // Emit the environment back upon request
  socket.on('env', function () {
    socket.emit('env', getEnv());
  })
});
