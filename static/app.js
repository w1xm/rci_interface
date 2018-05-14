angular.module('app', ['components'])
    .controller('StatusController', function($scope, $locale, RCI) {
	$scope.rci = RCI;
    })
    .controller('MapController', function($scope) {
	$scope.mapClick = function($event) {
	    let x = $event.offsetX-500;
	    let y = $event.offsetY-500;
	    let angle = Math.atan2(-y, x);
	    angle -= Math.PI/2;
	    if (angle < 0) { angle += 2*Math.PI; }
	    angle = -angle;
	    $scope.rci.setAzimuthPosition(360*angle/(2*Math.PI));
	};
	$scope.elClick = function($event) {
	    let x = $event.offsetX-10;
	    let y = $event.offsetY-90;
	    let angle = Math.atan2(y, x);
	    angle = -angle;
	    $scope.rci.setElevationPosition(360*angle/(2*Math.PI));
	};
    });
