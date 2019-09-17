angular.module('app', ['components', 'widgets', 'ngMaterial'])
    .config(function($mdGestureProvider) {
	$mdGestureProvider.skipClickHijack();
    })
    .controller('StatusController', function($scope, $locale, RCI) {
	$scope.rci = RCI;
	$scope.trackBody = 0;
	$scope.track = function() {
	    $scope.rci.track($scope.trackBody);
	};
	$scope.login = function() {
	    $scope.rci.reconnectWithPassword(prompt("Enter password"));
	};
	$scope.setAzElPosition = function($event) {
	    $scope.rci.setAzimuthPosition($event.az);
	    $scope.rci.setElevationPosition($event.el);
	};
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
    })
    .controller('PanoController', function($scope, RCI) {
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
    })
    .component('rciSkymap', {
	template: `
          <div ng-controller="StatusController" style="width: 100%; height: 100%;">
	    <skymap latitude="rci.status.Latitude" longitude="rci.status.Longitude" azel="[rci.status.AzPos, rci.status.ElPos, true]" targetazel="[rci.status.CommandAzPos, rci.status.CommandElPos, (rci.status.CommandAzFlags == 'POSITION' && rci.status.CommandElFlags == 'POSITION')]" click="setAzElPosition($event)"></skymap>
	  </div>`,
    });
