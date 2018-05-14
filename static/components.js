angular.module('components', [])
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
