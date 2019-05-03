module Views.TopBar exposing
    ( breadcrumbComponent
    , breadcrumbs
    , concourseLogo
    )

import Concourse
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , href
        , id
        )
import Message.Message exposing (Hoverable(..), Message(..))
import Routes
import Url
import Views.Styles as Styles
import Views.Views as Views


concourseLogo : Views.View Message
concourseLogo =
    Views.a Views.Unidentified Styles.concourseLogo [ href "/" ] []


breadcrumbs : Routes.Route -> Html Message
breadcrumbs route =
    mybreadcrumbs route |> Views.toHtml


mybreadcrumbs : Routes.Route -> Views.View Message
mybreadcrumbs route =
    Views.div
        (Views.Id "breadcrumbs")
        Styles.breadcrumbContainer
        []
    <|
        case route of
            Routes.Pipeline { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                ]

            Routes.Build { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , jobBreadcrumb id.jobName
                ]

            Routes.Resource { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , resourceBreadcrumb id.resourceName
                ]

            Routes.Job { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , jobBreadcrumb id.jobName
                ]

            _ ->
                []


breadcrumbComponent : String -> String -> List (Views.View Message)
breadcrumbComponent componentType name =
    [ Views.div
        Views.Unidentified
        (Styles.breadcrumbComponent componentType)
        []
        []
    , Views.text <| decodeName name
    ]


breadcrumbSeparator : Views.View Message
breadcrumbSeparator =
    Views.li
        Views.Unidentified
        (Styles.breadcrumbItem False)
        []
        [ Views.text "/" ]


pipelineBreadcrumb : Concourse.PipelineIdentifier -> Views.View Message
pipelineBreadcrumb pipelineId =
    Views.a
        (Views.Id "breadcrumb-pipeline")
        (Styles.breadcrumbItem True)
        [ href <|
            Routes.toString <|
                Routes.Pipeline { id = pipelineId, groups = [] }
        ]
        (breadcrumbComponent "pipeline" pipelineId.pipelineName)


jobBreadcrumb : String -> Views.View Message
jobBreadcrumb jobName =
    Views.li
        (Views.Id "breadcrumb-job")
        (Styles.breadcrumbItem False)
        []
        (breadcrumbComponent "job" jobName)


resourceBreadcrumb : String -> Views.View Message
resourceBreadcrumb resourceName =
    Views.li
        (Views.Id "breadcrumb-resource")
        (Styles.breadcrumbItem False)
        []
        (breadcrumbComponent "resource" resourceName)


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Url.percentDecode name)
