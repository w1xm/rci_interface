angular.module('app', ['components'])
    .controller('StatusController', function($scope, $locale, RCI) {
	$scope.rci = RCI;
    });
