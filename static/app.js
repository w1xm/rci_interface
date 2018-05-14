angular.module('app', ['components'])
    .controller('StatusController', function($scope, $locale, RCI) {
	$scope.rci = RCI;
	$scope.setAzimuthPosition = function($event) {
	    if (!$event.buttons) { return; }
	    let x = $event.offsetX-500;
	    let y = $event.offsetY-500;
	    let angle = Math.atan2(-y, x);
	    angle -= Math.PI/2;
	    if (angle < 0) { angle += 2*Math.PI; }
	    angle = -angle;
	    $scope.rci.setAzimuthPosition(360*angle/(2*Math.PI));
	};
    });
