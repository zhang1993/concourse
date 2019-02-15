describe('#draw', function() {
  var svg, resources, jobs, newUrl;

  beforeEach(function() {
    document.body.innerHTML = "<svg class=\"pipeline-graph\"></svg>"
    var foundSvg = d3.select('.pipeline-graph');
    svg = createPipelineSvg(foundSvg);
    resources = [
      {
        name: "resource",
        pipeline_name: "pipeline",
        team_name: "main",
        type: "mock",
        last_checked: 1550247797,
        pinned_version: {
          version: "version"
        }
      }
    ];
    jobs = [
        {
          id: 1,
          name: "job",
          pipeline_name: "pipeline",
          team_name: "main",
          next_build: null,
          finished_build: null,
          inputs: [
            {
            name: "resource",
            resource: "resource",
            trigger: false
            }
          ],
          outputs: [],
          groups: []
        }
      ];
    newUrl = { send: function() {} };
  });

  it('sets width to accomodate resource name and pin icon', function() {
    draw(svg, jobs, resources, newUrl);
    var resourceText = document.querySelector('.input text');
    var textWidth = resourceText.getBBox().width;
    var resourceIcon = document.querySelector('.input image');
    var iconWidth = resourceIcon.getBBox().width;
    var expectedWidth = 5 + textWidth + 5 + iconWidth + 5;
    var resourceRect = document.querySelector('.input rect');
    expect(resourceRect.getAttribute('width')).toBe(expectedWidth.toString());
  });

  describe('when job has input with trigger: false', function() {
    beforeEach(function() {
      jobs = [
        {
          id: 1,
          name: "job",
          pipeline_name: "pipeline",
          team_name: "main",
          next_build: null,
          finished_build: null,
          inputs: [
            {
            name: "resource",
            resource: "resource",
            trigger: false
            }
          ],
          outputs: [],
          groups: []
        }
      ];
    });

    it('adds trigger-false class to edge', function() {
      draw(svg, jobs, resources, newUrl);
      var edge = document.querySelector('.edge');
      expect(edge.classList.contains('trigger-false')).toBe(true);
    });
  });

  describe('when job has input with trigger: true', function() {
    beforeEach(function() {
      jobs = [
        {
          id: 1,
          name: "job",
          pipeline_name: "pipeline",
          team_name: "main",
          next_build: null,
          finished_build: null,
          inputs: [
            {
            name: "resource",
            resource: "resource",
            trigger: true
            }
          ],
          outputs: [],
          groups: []
        }
      ];
    });

    it('does not add trigger-false class to edge', function() {
      draw(svg, jobs, resources, newUrl);
      var edge = document.querySelector('.edge');
      expect(edge.classList.contains('trigger-false')).toBe(false);
    });
  });
});
