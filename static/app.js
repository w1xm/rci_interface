angular.module('app', ['components'])
    .controller('StatusController', function($scope, $locale, RCI) {
	$scope.rci = RCI;
    })
    .controller('MapController', function($scope) {
	$scope.mapClick = function($event) {
	    var rect = $event.currentTarget.ownerSVGElement.getBoundingClientRect();
	    let x = ($event.clientX-rect.left)-500;
	    let y = ($event.clientY-rect.top)-500;
	    let angle = Math.atan2(-y, x);
	    angle -= Math.PI/2;
	    if (angle < 0) { angle += 2*Math.PI; }
	    angle = -angle;
	    angle = 360*angle/(2*Math.PI);
	    $scope.rci.setAzimuthPosition(angle);
	};
	$scope.elClick = function($event) {
	    var rect = $event.currentTarget.ownerSVGElement.getBoundingClientRect();
	    let x = ($event.clientX-rect.left)-10;
	    let y = ($event.clientY-rect.top)-90;
	    let angle = Math.atan2(y, x);
	    angle = -angle;
	    $scope.rci.setElevationPosition(360*angle/(2*Math.PI));
	};
    });
