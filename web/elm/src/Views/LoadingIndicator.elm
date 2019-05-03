module Views.LoadingIndicator exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (class)
import Views.Spinner as Spinner
import Views.Views as Views exposing (style)


view : Views.View x
view =
    Views.div Views.Unidentified
        []
        [ class "build-step" ]
        [ Views.div Views.Unidentified
            [ style "display" "flex" ]
            [ class "header" ]
            [ Spinner.spinner { size = "14px", margin = "7px" }
            , Views.h3 Views.Unidentified [] [] [ Views.text "loading..." ]
            ]
        ]
