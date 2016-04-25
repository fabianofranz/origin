'use strict';

angular.module('openshiftConsole')
  .factory('Jenkins', function($q, DataService, RoutesService, routeWebURLFilter, $routeParams) {
    // https://github.com/openshift/origin/pull/7949/files#diff-d59c94963a713e75e3b30c98b1dc40f2R42
    var jenkinsDefaultServiceName = "jenkins";

    var jenkinsURL;
    function discoverJenkinsURL(projectName) {
      if (angular.isDefined(jenkinsURL)) {
        return $q.when(jenkinsURL);
      }

      var context = {};

      DataService.get("services", jenkinsDefaultServiceName, context).then(
        function(jenkinsService) {
          var preferredRoute;

          DataService.list("routes", context, function(routes) {
            angular.forEach(routes.by("metadata.name"), function(route, routeName){
              if (route.spec.to.kind !== "Service") {
                return;
              }
              if (route.spec.to.name === jenkinsService.metadata.name) {
                if (angular.isDefined(preferredRoute)) {
                  preferredRoute = route;
                } else {
                  preferredRoute = RoutesService.getPreferredDisplayRoute(preferredRoute, route);
                }
              }
            });
          });

          jenkinsURL = routeWebURLFilter(preferredRoute);
        },
        function(e) {
          jenkinsURL = null;
        }
      );
    }

    return {
      discoverJenkinsURL: discoverJenkinsURL,
      jenkinsDefined: function(projectName) {
        return discoverJenkinsURL() !== null;
      }
    };
  });
