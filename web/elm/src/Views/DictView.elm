module Views.DictView exposing (view)

import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class)
import Views.Views as Views


view : List (Html.Attribute a) -> Dict String (Views.View a) -> Views.View a
view attrs dict =
    Views.table Views.Unidentified [] (class "dictionary" :: attrs) <|
        List.map viewPair (Dict.toList dict)


viewPair : ( String, Views.View a ) -> Views.View a
viewPair ( name, value ) =
    Views.tr Views.Unidentified
        []
        []
        [ Views.td Views.Unidentified [] [ class "dict-key" ] [ Views.text name ]
        , Views.td Views.Unidentified [] [ class "dict-value" ] [ value ]
        ]
