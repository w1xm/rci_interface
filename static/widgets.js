angular.module('widgets', [])
    .directive('posvel', function() {
	return {
	    templateUrl: 'posvel.html',
	    scope: {
		'flags': '<',
		'pos': '<',
		'vel': '<',
		'onPos': '&',
		'onVel': '&',
		'writable': '<',
	    }
	};
    })
    .directive('knob', function() {
	return {
	    template: '',
	    restrict: 'E',
	    scope: {
		'value': '=ngModel',
		'writable': '<',
		'max': '<',
		'min': '<',
		'wrap': '<',
		'unit': '@',
		'active': '<',
	    },
	    require: 'ngModel',
	    link: function(scope, element, attrs, ngModelCtrl) {
		const writable = scope.writable;

		const decimals = 3;

		// TODO: use integer flag of RangeT, w decimal points?
		function clamp(value, direction) {
		    if (scope.wrap) {
			while (value >= scope.max) {
			    value -= (scope.max-scope.min);
			}
			while (value < scope.min) {
			    value += (scope.max-scope.min);
			}
			return value;
		    } else {
			if (scope.max && value >= scope.max) {
			    return scope.max;
			}
			if (scope.min && value < scope.min) {
			    return scope.min;
			}
			return value;
		    }
		}

		const container = document.createElement('span');
		container.classList.add('widget-Knob-outer');

		element.append(container);

		const places = [];
		const marks = [];
		const fixedMarks = [];
		function createPlace(i, idx) {
		    if (i > 0 && i % 3 === 2 && places.length > 0) {
			const mark = container.appendChild(document.createElement("span"));
			mark.className = "knob-mark";
			mark.textContent = ",";
			//mark.style.visibility = "hidden";
			marks.unshift(mark);
			// TODO: make marks responsive to scroll events (doesn't matter which neighbor, or split in the middle, as long as they do something).
		    }
		    if (i === -1) {
			const mark = container.appendChild(document.createElement("span"));
			mark.className = "knob-mark";
			mark.textContent = ".";
			//mark.style.visibility = "hidden";
			fixedMarks.unshift(mark)
		    }
		    const digit = container.appendChild(document.createElement("span"));
		    digit.className = "knob-digit";
		    const digitText = digit.appendChild(document.createTextNode('0'));
		    places[idx] = {element: digit, text: digitText};
		    const scale = Math.pow(10, i);

		    if (!writable) return;

		    digit.tabIndex = -1;

		    function spin(direction) {
			ngModelCtrl.$setViewValue(clamp(direction * scale + ngModelCtrl.$viewValue, direction));
			ngModelCtrl.$render();
		    }
		    digit.addEventListener('wheel', event => {
			// TODO: deal with high-res/accelerated scrolling
			spin((event.deltaY || event.deltaX) > 0 ? 1 : -1);
			event.preventDefault();
			event.stopPropagation();
		    }, {capture: true, passive: false});
		    function focusNext() {
			if (idx > 0) {
			    places[idx - 1].element.focus();
			} else {
			    //digit.blur();
			}
		    }
		    function focusPrev() {
			if (idx < places.length - 1) {
			    places[idx + 1].element.focus();
			} else {
			    //digit.blur();
			}
		    }
		    digit.addEventListener('keydown', event => {
			switch (event.keyCode) {  // nominally poorly compatible, but best we can do
			case 0x08: // backspace
			case 0x25: // left
			    focusPrev();
			    break;
			case 0x27: // right
			    focusNext();
			    break;
			case 0x26: // up
			    spin(1);
			    break;
			case 0x28: // down
			    spin(-1);
			    break;
			default:
			    return;
			}
			event.preventDefault();
			event.stopPropagation();
		    }, true);
		    digit.addEventListener('keypress', event => {
			var ch = String.fromCharCode(event.charCode);
			var value = ngModelCtrl.$viewValue;

			switch (ch) {
			case '-':
			case '_':
			    ngModelCtrl.$setViewValue(-Math.abs(value));
			    ngModelCtrl.$render();
			    return;
			case '+':
			case '=':
			    ngModelCtrl.$setViewValue(Math.abs(value));
			    ngModelCtrl.$render();
			    return;
			case 'z':
			case 'Z':
			    // zero all digits here and to the right
			    // | 0 is used to round towards zero
			    var zeroFactor = scale * 10;
			    ngModelCtrl.$setViewValue(((value / zeroFactor) | 0) * zeroFactor);
			    ngModelCtrl.$render();
			    return;
			default:
			    break;
			}

			// TODO I hear there's a new 'input' event which is better for input-ish keystrokes, use that
			var input = parseInt(ch, 10);
			if (isNaN(input)) return;

			var negative = value < 0 || (value === 0 && 1/value === -Infinity);
			if (negative) { value = -value; }
			var currentDigitValue;
			if (scale === 1) {
			    // When setting last digit, clear anyT hidden fractional digits as well
			    currentDigitValue = (value / scale) % 10;
			} else {
			    currentDigitValue = Math.floor(value / scale) % 10;
			}
			value += (input - currentDigitValue) * scale;
			if (negative) { value = -value; }
			ngModelCtrl.$setViewValue(clamp(value, 0));
			ngModelCtrl.$render();

			focusNext();
			event.preventDefault();
			event.stopPropagation();
		    });

		    // remember last place for tabbing
		    digit.addEventListener('focus', event => {
			places.forEach(other => {
			    other.element.tabIndex = -1;
			});
			digit.tabIndex = 0;
		    }, false);

		    // spin buttons
		    digit.style.position = 'relative';
		    [-1, 1].forEach(direction => {
			var up = direction > 0;
			var layoutShim = digit.appendChild(document.createElement('span'));
			layoutShim.className = 'knob-spin-button-shim knob-spin-' + (up ? 'up' : 'down');
			var button = layoutShim.appendChild(document.createElement('button'));
			button.className = 'knob-spin-button knob-spin-' + (up ? 'up' : 'down');
			button.textContent = up ? '+' : '-';
			function pushListener(event) {
			    spin(direction);
			    event.preventDefault();
			    event.stopPropagation();
			}
			// Using these events instead of click event allows the button to work despite the auto-hide-on-focus-loss, in Chrome.
			button.addEventListener('touchstart', pushListener, {capture: true, passive: false});
			button.addEventListener('mousedown', pushListener, {capture: true, passive: false});
			//button.addEventListener('click', pushListener, false);
			// If in the normal tab order, its appearing/disappearing causes trouble
			button.tabIndex = -1;
		    });
		}

		for (let i = 2; i >= -decimals; i--) {
		    createPlace(i, i+decimals);
		}

		if (scope.unit) {
		    const mark = container.appendChild(document.createElement("span"));
		    mark.className = "knob-mark";
		    mark.textContent = scope.unit;
		    fixedMarks.unshift(mark);
		}

		places[places.length - 1].element.tabIndex = 0; // initial tabbable digit

		ngModelCtrl.$render = function() {
		    const value = ngModelCtrl.$viewValue;
		    //console.log('externally changed to', value);
		    const active = angular.isDefined(scope.active) ? scope.active: true;
		    let valueStr = new Intl.NumberFormat('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals, useGrouping: false }).format(value);
		    if (valueStr === '0' && value === 0 && 1/value === -Infinity) {
			// allow user to see progress in entering negative values
			valueStr = '-0';
		    }
		    const valueStrDigits = valueStr.replace(".", "");
		    const last = valueStrDigits.length - 1;
		    for (let i = 0; i < places.length; i++) {
			const digit = valueStrDigits[last - i];
			places[i].text.data = digit || '0';
			places[i].element.classList[(digit && active) ? 'remove' : 'add']('knob-dim');
		    }
		    const numMarks = Math.floor((valueStrDigits.replace("-", "").length - 1 - 2) / 3);
		    for (let i = 0; i < marks.length; i++) {
			marks[i].classList[(i < numMarks && active) ? 'remove' : 'add']('knob-dim');
		    }
		    for (let i = 0; i < fixedMarks.length; i++) {
			fixedMarks[i].classList[active ? 'remove' : 'add']('knob-dim');
		    }
		};
		scope.$watch('active', ngModelCtrl.$render);
	    },
	}
    })
    .directive('skymap', function($timeout) {
	let nextId = 0;
	return {
	    template: '',
	    restrict: 'E',
	    scope: {
		'latitude': '<',
		'longitude': '<',
		'azel': '<',
		'targetazel': '<',
		'click': '&?',
	    },
	    link: function(scope, element, attrs) {
		const container = document.createElement('div');
		container.id = 'skymap-' + (nextId++)
		container.style.width = container.style.height = "100%";
		element.append(container);

		const planetarium = S.virtualsky({
		    'id': container.id,
		    'projection': 'stereo',
		    'fov': 45,
		    'az': scope.azel[0],
		    'live': true,
		    'showplanets': true,
		    'showstarlabels': true,
		    'gridlines_az': true,
		    'showgalaxy': true,
		    'mouse': false,
		    'objects': 'virtualsky/messier.json',
		    'callback': {
			'click': function(e) {
			    let azel = this.projection.xy2azel(e.x, e.y, this.wide, this.tall);
			    e.az = azel[0]/this.d2r+this.az_off;
			    e.el = azel[1]/this.d2r;
			    if (scope.click) {
				scope.click({'$event': e});
			    }
			},
		    },
		});
		$timeout(function() {
		    planetarium.resize();
		});

		const pointerStatus = planetarium.addPointer({
		    ra: 0,
		    dec: 0,
		    label: 'status',
		    d: 1.5, // degrees
		    colour: planetarium.col.pointers,
		})-1;
		const pointerTarget = planetarium.addPointer({
		    ra: 0,
		    dec: 0,
		    d: 10, // pixels
		    label: 'target',
		    colour: 'red',
		})-1;

		oldHighlight = planetarium.highlight;
		planetarium.highlight = function(i) {
		    const p = this.pointers[i];
		    if (p.ra && p.dec) {
			const pos = this.radec2xy(p.ra*this.d2r, p.dec*this.d2r);
			if (i == pointerTarget) {
			    // Draw a crosshair
			    const c = this.ctx;
			    c.beginPath();
			    c.strokeStyle = p.colour;
			    c.moveTo(pos.x-p.d, pos.y);
			    c.lineTo(pos.x-1, pos.y);
			    c.moveTo(pos.x+1, pos.y);
			    c.lineTo(pos.x+p.d, pos.y);
			    c.moveTo(pos.x, pos.y-p.d);
			    c.lineTo(pos.x, pos.y-1);
			    c.moveTo(pos.x, pos.y+1);
			    c.lineTo(pos.x, pos.y+p.d);
			    c.stroke();
			} else if (i == pointerStatus) {
			    // Draw a circle
			    let radius = Math.abs(pos.x-this.radec2xy((p.ra-(p.d/2))*this.d2r, p.dec*this.d2r).x);
			    const c = this.ctx;
			    c.beginPath();
			    c.strokeStyle = p.colour;
			    c.arc(pos.x, pos.y, radius, 0, 2*Math.PI);
			    c.stroke();
			} else {
			    oldHighlight.call(this, i);
			}
		    }
		};

		scope.$watch('latitude', function(lat) {
		    planetarium.setLatitude(lat);
		});
		scope.$watch('longitude', function(lon) {
		    planetarium.setLongitude(lon);
		});
		let updateAzel = function(pointer, azel) {
		    if (!azel[2]) {
			planetarium.pointers[pointer].ra = planetarium.pointers[pointer].dec = 0;
			planetarium.draw();
			return;
		    }
		    planetarium.az_off = azel[0]%360-180;
		    let radec = planetarium.azel2radec(azel[0]*planetarium.d2r, azel[1]*planetarium.d2r);
		    planetarium.pointers[pointer].ra = radec.ra;
		    planetarium.pointers[pointer].dec = radec.dec;
		    planetarium.draw();
		};
		scope.$watch('azel', updateAzel.bind(undefined, pointerStatus));
		scope.$watch('targetazel', updateAzel.bind(undefined, pointerTarget));
	    },
	};
    })
    .directive('pano', function() {
	return {
	    restrict: 'E',
	    templateUrl: 'widgets/pano.html',
	    controller: function($scope, RCI) {
		$scope.rci = RCI;
		$scope.pixelsPerDegree = 15.6;
		$scope.horizonOffset = 320;
		$scope.panoShift = 0;
		$scope.$watch('rci.status.AzPos', n => { $scope.panoShift = 500 - (n * $scope.pixelsPerDegree) });
		$scope.commandY = 0;
		$scope.$watch('rci.status.CommandElPos', n => { $scope.commandY = $scope.horizonOffset-(((n+180)%360-180) * $scope.pixelsPerDegree) });
		$scope.commandX = 0;
		$scope.$watch('rci.status.CommandAzPos', n => { $scope.commandX = n * $scope.pixelsPerDegree });
		$scope.statusY = 0;
		$scope.$watch('rci.status.ElPos', n => { $scope.statusY = $scope.horizonOffset - (((n+180)%360-180) * $scope.pixelsPerDegree) });
		$scope.panoClick = function($event) {
		    var rect = $event.currentTarget.ownerSVGElement.getBoundingClientRect();
		    let az = ($event.clientX - rect.left - $scope.panoShift) / $scope.pixelsPerDegree;
		    let el = ($scope.horizonOffset - ($event.clientY - rect.top)) / $scope.pixelsPerDegree;
		    RCI.setAzimuthPosition(az);
		    RCI.setElevationPosition(el);
		};
		$scope.ticks = function() {
		    let out = []
		    for (let i = 0; i < 360; i+=10) {
			let major = (i % 30) == 0;
			out.push({
			    angle: i,
			    x: i * $scope.pixelsPerDegree,
			    height: major ? 20 : 10,
			    major: major,
			});
		    }
		    return out;
		}();
	    },
	};
    });
