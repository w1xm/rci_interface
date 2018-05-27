angular.module('components', [
    'ngWebSocket',
])
    .factory('RCI', function($websocket, $window) {
	var obj = {
	    status: {},
	}
	obj.write = function(register, values) {
	    obj.socket.send(JSON.stringify({
		command: 'write',
		register: register,
		values: values,
	    }));
	};
	obj.setAzimuthPosition = function(position) {
	    obj.socket.send(JSON.stringify({
		command: 'set_azimuth_position',
		position: position,
	    }));
	};
	obj.setElevationPosition = function(position) {
	    obj.socket.send(JSON.stringify({
		command: 'set_elevation_position',
		position: position,
	    }));
	};
	obj.setAzimuthVelocity = function(velocity) {
	    obj.socket.send(JSON.stringify({
		command: 'set_azimuth_velocity',
		velocity: velocity,
	    }));
	};
	obj.setElevationVelocity = function(velocity) {
	    obj.socket.send(JSON.stringify({
		command: 'set_elevation_velocity',
		velocity: velocity,
	    }));
	};
	obj.setAzimuthOffset = function(position) {
	    obj.socket.send(JSON.stringify({
		command: 'set_azimuth_offset',
		position: position,
	    }));
	};
	obj.setElevationOffset = function(position) {
	    obj.socket.send(JSON.stringify({
		command: 'set_elevation_offset',
		position: position,
	    }));
	};
	obj.track = function(body) {
	    obj.socket.send(JSON.stringify({
		command: 'track',
		body: body,
	    }));
	};
	obj.stop = function() {
	    obj.socket.send(JSON.stringify({
		command: 'stop',
	    }));
	};
	obj.stopHard = function() {
	    obj.socket.send(JSON.stringify({
		command: 'stop_hard',
	    }));
	};
	obj.exitShutdown = function() {
	    obj.socket.send(JSON.stringify({
		command: 'exit_shutdown',
	    }));
	};

	obj.reconnectWithPassword = function(password) {
	    let host = $window.location.host;
	    if (obj.socket) {
		obj.socket.socket.close(1000);
	    }
	    let protocols = undefined;
	    if (password) {
		protocols = [password];
	    }
	    // Open a WebSocket connection
	    obj.socket = $websocket(
		'ws://'+host+'/api/ws', protocols, {
		    reconnectIfNotNormalClose: true,
		});

	    obj.socket.onMessage(function(message) {
		obj.status = JSON.parse(message.data);
	    });
	};

	obj.reconnectWithPassword(null);

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
