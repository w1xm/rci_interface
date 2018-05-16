angular.module('widgets', [])
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
	    },
	    require: 'ngModel',
	    link: function(scope, element, attrs, ngModelCtrl) {
		const writable = scope.writable;

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
			    return;
			case '+':
			case '=':
			    ngModelCtrl.$setViewValue(Math.abs(value));
			    return;
			case 'z':
			case 'Z':
			    // zero all digits here and to the right
			    // | 0 is used to round towards zero
			    var zeroFactor = scale * 10;
			    ngModelCtrl.$setViewValue(((value / zeroFactor) | 0) * zeroFactor);
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

		for (let i = 2; i >= -2; i--) {
		    createPlace(i, i+2);
		}

		if (scope.unit) {
		    const mark = container.appendChild(document.createElement("span"));
		    mark.className = "knob-mark";
		    mark.textContent = scope.unit;
		}

		places[places.length - 1].element.tabIndex = 0; // initial tabbable digit

		ngModelCtrl.$render = function() {
		    const value = ngModelCtrl.$viewValue;
		    console.log('value changed remotely', value);
		    let valueStr = new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2, useGrouping: false }).format(value);
		    if (valueStr === '0' && value === 0 && 1/value === -Infinity) {
			// allow user to see progress in entering negative values
			valueStr = '-0';
		    }
		    const valueStrDigits = valueStr.replace(".", "");
		    const last = valueStrDigits.length - 1;
		    for (let i = 0; i < places.length; i++) {
			const digit = valueStrDigits[last - i];
			places[i].text.data = digit || '0';
			places[i].element.classList[digit ? 'remove' : 'add']('knob-dim');
		    }
		    const numMarks = Math.floor((valueStrDigits.replace("-", "").length - 1 - 2) / 3);
		    for (let i = 0; i < marks.length; i++) {
			marks[i].classList[i < numMarks ? 'remove' : 'add']('knob-dim');
		    }
		};
	    },
	}
    })
