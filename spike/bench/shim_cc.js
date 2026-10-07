try { require('module').enableCompileCache && require('module').enableCompileCache(require('path').join(__dirname, 'cc')); } catch (e) {}
require('./dist/shim.bundle.js');
