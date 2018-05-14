angular.module('components', [
    'ngWebSocket',
])
    .factory('RCI', function($websocket, $window) {
	// Open a WebSocket connection
	var socket = $websocket('ws://'+$window.location.host+'/api/ws');

	var obj = {
	    status: {},
	    write: function(register, values) {
		socket.send(JSON.stringify({
		    command: 'write',
		    register: register,
		    values: values,
		}));
	    },
	    setAzimuthPosition: function(position) {
		socket.send(JSON.stringify({
		    command: 'set_azimuth_position',
		    position: position,
		}));
	    },
	    setElevationPosition: function(position) {
		socket.send(JSON.stringify({
		    command: 'set_elevation_position',
		    position: position,
		}));
	    },
	    stop: function() {
		socket.send(JSON.stringify({
		    command: 'stop',
		}));
	    },
	};

	socket.onMessage(function(message) {
            obj.status = JSON.parse(message.data);
	});

	return obj;
    })
    .filter('bits', function() {
	return function(input) {
	    if (!input) {
		return '';
	    }
	    var out = '';
	    for (var i = 0; i < input.length; i++) {
		out = int(input[i]) + out;
	    }
	    return out;
	}
    })
