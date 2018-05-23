angular.module('components', [
    'ngWebSocket',
])
    .factory('RCI', function($websocket, $window) {
	// Open a WebSocket connection
	var socket = $websocket(
	    'ws://'+$window.location.host+'/api/ws', {
		reconnectIfNotNormalClose: true,
	    });

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
	    setAzimuthVelocity: function(velocity) {
		socket.send(JSON.stringify({
		    command: 'set_azimuth_velocity',
		    velocity: velocity,
		}));
	    },
	    setElevationVelocity: function(velocity) {
		socket.send(JSON.stringify({
		    command: 'set_elevation_velocity',
		    velocity: velocity,
		}));
	    },
	    setAzimuthOffset: function(position) {
		socket.send(JSON.stringify({
		    command: 'set_azimuth_offset',
		    position: position,
		}));
	    },
	    setElevationOffset: function(position) {
		socket.send(JSON.stringify({
		    command: 'set_elevation_offset',
		    position: position,
		}));
	    },
	    track: function(body) {
		socket.send(JSON.stringify({
		    command: 'track',
		    body: body,
		}));
	    },
	    stop: function() {
		socket.send(JSON.stringify({
		    command: 'stop',
		}));
	    },
	    stopHard: function() {
		socket.send(JSON.stringify({
		    command: 'stop_hard',
		}));
	    },
	    exitShutdown: function() {
		socket.send(JSON.stringify({
		    command: 'exit_shutdown',
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
		out = (1*input[i]) + out;
	    }
	    return out;
	}
    })
    .filter('deg', function() {
	return function(input) {
	    if (input===undefined) {
		return '';
	    }
	    return new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2, useGrouping: false }).format(input);
	}
    })
    .filter('hex', function() {
	let f = function(input) {
	    if (input===undefined) {
		return '';
	    }
	    if (input.length) {
		return '['+input.map(f).join(', ')+']'
	    }
	    return input.toString(16);
	};
	return f;
    })
    .filter('shutdown', function() {
	const errors = [
	    "None",
	    "RAM write/read test failure",
	    "Azimuth A/D not done",
	    "Elevation A/D not done",
	    "Azimuth tach inconsistent",
	    "Elevation tach inconsistent",
	    "Upper Elevation limit",
	    "Lower Elevation limit",
	    "Unresponsive Azimuth",
	    "Unresponsive Elevation",
	    "Azimuth overvelocity",
	    "Elevation overvelocity",
	    "Elevation position out of range",
	];
	return function(input) {
	    return errors[input];
	};
    })
